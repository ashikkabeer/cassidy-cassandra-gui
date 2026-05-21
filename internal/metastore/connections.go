package metastore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Connection is the persisted shape of a saved Cassandra connection. Secrets
// (auth_password_enc, tls_client_key_enc) are AES-GCM ciphertext blobs; the
// repo never touches plaintext — callers in the service layer decrypt as needed.
type Connection struct {
	ID               string
	OwnerID          string
	Name             string
	Hosts            []string
	Port             int
	Datacenter       sql.NullString
	DefaultKeyspace  sql.NullString
	AuthUsername     sql.NullString
	AuthPasswordEnc  []byte // nil = no auth
	TLSEnabled       bool
	TLSSkipVerify    bool
	TLSCACert        sql.NullString
	TLSClientCert    sql.NullString
	TLSClientKeyEnc  []byte
	ReadOnly         bool
	Consistency      string
	ConnectTimeoutMS int
	RequestTimeoutMS int
	CreatedAt        time.Time
	UpdatedAt        time.Time
	LastUsedAt       sql.NullTime
}

type Connections struct{ db *sql.DB }

func NewConnections(db *sql.DB) *Connections { return &Connections{db: db} }

var ErrConnectionNotFound = errors.New("connection not found")

// CreateConnectionParams is what callers pass to the repo. All secrets MUST
// already be encrypted; the repo writes the bytes as-is.
type CreateConnectionParams struct {
	OwnerID          string
	Name             string
	Hosts            []string
	Port             int
	Datacenter       string
	DefaultKeyspace  string
	AuthUsername     string
	AuthPasswordEnc  []byte
	TLSEnabled       bool
	TLSSkipVerify    bool
	TLSCACert        string
	TLSClientCert    string
	TLSClientKeyEnc  []byte
	ReadOnly         bool
	Consistency      string
	ConnectTimeoutMS int
	RequestTimeoutMS int
}

func (c *Connections) Create(ctx context.Context, p CreateConnectionParams) (*Connection, error) {
	if p.OwnerID == "" {
		return nil, fmt.Errorf("owner_id is required")
	}
	if strings.TrimSpace(p.Name) == "" {
		return nil, fmt.Errorf("name is required")
	}
	if len(p.Hosts) == 0 {
		return nil, fmt.Errorf("at least one host is required")
	}
	hostsJSON, err := json.Marshal(p.Hosts)
	if err != nil {
		return nil, fmt.Errorf("marshal hosts: %w", err)
	}
	id := uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = c.db.ExecContext(ctx, `
		INSERT INTO connections (
			id, owner_id, name, hosts, port, datacenter, default_keyspace,
			auth_username, auth_password_enc,
			tls_enabled, tls_skip_verify, tls_ca_cert, tls_client_cert, tls_client_key_enc,
			read_only, consistency, connect_timeout_ms, request_timeout_ms,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, p.OwnerID, p.Name, string(hostsJSON), p.Port,
		nullableString(p.Datacenter), nullableString(p.DefaultKeyspace),
		nullableString(p.AuthUsername), nullableBytes(p.AuthPasswordEnc),
		boolToInt(p.TLSEnabled), boolToInt(p.TLSSkipVerify),
		nullableString(p.TLSCACert), nullableString(p.TLSClientCert), nullableBytes(p.TLSClientKeyEnc),
		boolToInt(p.ReadOnly), p.Consistency, p.ConnectTimeoutMS, p.RequestTimeoutMS,
		now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert connection: %w", err)
	}
	return c.GetByID(ctx, id)
}

func (c *Connections) GetByID(ctx context.Context, id string) (*Connection, error) {
	row := c.db.QueryRowContext(ctx, selectConnectionSQL+` WHERE id = ?`, id)
	return scanConnection(row)
}

// GetForOwner is the safer variant: returns ErrConnectionNotFound if the row
// exists but is owned by someone else. Callers that route on the wire should
// use this so non-owners can't probe IDs.
func (c *Connections) GetForOwner(ctx context.Context, ownerID, id string) (*Connection, error) {
	row := c.db.QueryRowContext(ctx, selectConnectionSQL+` WHERE id = ? AND owner_id = ?`, id, ownerID)
	return scanConnection(row)
}

func (c *Connections) ListByOwner(ctx context.Context, ownerID string) ([]Connection, error) {
	rows, err := c.db.QueryContext(ctx, selectConnectionSQL+`
		WHERE owner_id = ? ORDER BY datetime(updated_at) DESC, name ASC`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list connections: %w", err)
	}
	defer rows.Close()
	var out []Connection
	for rows.Next() {
		conn, err := scanConnection(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *conn)
	}
	return out, rows.Err()
}

// UpdateConnectionParams uses pointer fields so callers can express "retain"
// (nil) vs "set to this value" (non-nil). Secret fields work the same way:
// `AuthPasswordEnc == nil` → retain; `AuthPasswordEnc == &[]byte{}` → clear;
// non-empty bytes → re-encrypt.
type UpdateConnectionParams struct {
	Name             *string
	Hosts            *[]string
	Port             *int
	Datacenter       *string
	DefaultKeyspace  *string
	AuthUsername     *string
	AuthPasswordEnc  *[]byte
	TLSEnabled       *bool
	TLSSkipVerify    *bool
	TLSCACert        *string
	TLSClientCert    *string
	TLSClientKeyEnc  *[]byte
	ReadOnly         *bool
	Consistency      *string
	ConnectTimeoutMS *int
	RequestTimeoutMS *int
}

func (c *Connections) Update(ctx context.Context, ownerID, id string, p UpdateConnectionParams) error {
	sets, args, err := buildConnUpdate(p)
	if err != nil {
		return err
	}
	if len(sets) == 0 {
		return nil
	}
	sets = append(sets, "updated_at = ?")
	args = append(args, time.Now().UTC().Format(time.RFC3339Nano))
	args = append(args, id, ownerID)
	q := "UPDATE connections SET " + strings.Join(sets, ", ") + " WHERE id = ? AND owner_id = ?"
	res, err := c.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("update connection: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrConnectionNotFound
	}
	return nil
}

func (c *Connections) Delete(ctx context.Context, ownerID, id string) error {
	res, err := c.db.ExecContext(ctx,
		`DELETE FROM connections WHERE id = ? AND owner_id = ?`, id, ownerID,
	)
	if err != nil {
		return fmt.Errorf("delete connection: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrConnectionNotFound
	}
	return nil
}

func (c *Connections) TouchLastUsed(ctx context.Context, id string) {
	_, _ = c.db.ExecContext(ctx,
		`UPDATE connections SET last_used_at = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339Nano), id,
	)
}

const selectConnectionSQL = `
	SELECT id, owner_id, name, hosts, port, datacenter, default_keyspace,
	       auth_username, auth_password_enc,
	       tls_enabled, tls_skip_verify, tls_ca_cert, tls_client_cert, tls_client_key_enc,
	       read_only, consistency, connect_timeout_ms, request_timeout_ms,
	       created_at, updated_at, last_used_at
	FROM connections`

func scanConnection(row interface{ Scan(...any) error }) (*Connection, error) {
	var c Connection
	var hosts string
	var tlsEnabled, tlsSkip, readOnly int
	var createdAt, updatedAt string
	var lastUsedAt sql.NullString
	err := row.Scan(
		&c.ID, &c.OwnerID, &c.Name, &hosts, &c.Port, &c.Datacenter, &c.DefaultKeyspace,
		&c.AuthUsername, &c.AuthPasswordEnc,
		&tlsEnabled, &tlsSkip, &c.TLSCACert, &c.TLSClientCert, &c.TLSClientKeyEnc,
		&readOnly, &c.Consistency, &c.ConnectTimeoutMS, &c.RequestTimeoutMS,
		&createdAt, &updatedAt, &lastUsedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrConnectionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan connection: %w", err)
	}
	if err := json.Unmarshal([]byte(hosts), &c.Hosts); err != nil {
		return nil, fmt.Errorf("decode hosts: %w", err)
	}
	c.TLSEnabled = tlsEnabled == 1
	c.TLSSkipVerify = tlsSkip == 1
	c.ReadOnly = readOnly == 1
	c.CreatedAt = parseTime(createdAt)
	c.UpdatedAt = parseTime(updatedAt)
	if lastUsedAt.Valid {
		c.LastUsedAt = sql.NullTime{Time: parseTime(lastUsedAt.String), Valid: true}
	}
	return &c, nil
}

func buildConnUpdate(p UpdateConnectionParams) ([]string, []any, error) {
	var sets []string
	var args []any
	if p.Name != nil {
		sets = append(sets, "name = ?")
		args = append(args, *p.Name)
	}
	if p.Hosts != nil {
		if len(*p.Hosts) == 0 {
			return nil, nil, fmt.Errorf("hosts cannot be empty")
		}
		b, err := json.Marshal(*p.Hosts)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal hosts: %w", err)
		}
		sets = append(sets, "hosts = ?")
		args = append(args, string(b))
	}
	if p.Port != nil {
		sets = append(sets, "port = ?")
		args = append(args, *p.Port)
	}
	if p.Datacenter != nil {
		sets = append(sets, "datacenter = ?")
		args = append(args, nullableString(*p.Datacenter))
	}
	if p.DefaultKeyspace != nil {
		sets = append(sets, "default_keyspace = ?")
		args = append(args, nullableString(*p.DefaultKeyspace))
	}
	if p.AuthUsername != nil {
		sets = append(sets, "auth_username = ?")
		args = append(args, nullableString(*p.AuthUsername))
	}
	if p.AuthPasswordEnc != nil {
		sets = append(sets, "auth_password_enc = ?")
		args = append(args, nullableBytes(*p.AuthPasswordEnc))
	}
	if p.TLSEnabled != nil {
		sets = append(sets, "tls_enabled = ?")
		args = append(args, boolToInt(*p.TLSEnabled))
	}
	if p.TLSSkipVerify != nil {
		sets = append(sets, "tls_skip_verify = ?")
		args = append(args, boolToInt(*p.TLSSkipVerify))
	}
	if p.TLSCACert != nil {
		sets = append(sets, "tls_ca_cert = ?")
		args = append(args, nullableString(*p.TLSCACert))
	}
	if p.TLSClientCert != nil {
		sets = append(sets, "tls_client_cert = ?")
		args = append(args, nullableString(*p.TLSClientCert))
	}
	if p.TLSClientKeyEnc != nil {
		sets = append(sets, "tls_client_key_enc = ?")
		args = append(args, nullableBytes(*p.TLSClientKeyEnc))
	}
	if p.ReadOnly != nil {
		sets = append(sets, "read_only = ?")
		args = append(args, boolToInt(*p.ReadOnly))
	}
	if p.Consistency != nil {
		sets = append(sets, "consistency = ?")
		args = append(args, *p.Consistency)
	}
	if p.ConnectTimeoutMS != nil {
		sets = append(sets, "connect_timeout_ms = ?")
		args = append(args, *p.ConnectTimeoutMS)
	}
	if p.RequestTimeoutMS != nil {
		sets = append(sets, "request_timeout_ms = ?")
		args = append(args, *p.RequestTimeoutMS)
	}
	return sets, args, nil
}

func nullableBytes(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}
