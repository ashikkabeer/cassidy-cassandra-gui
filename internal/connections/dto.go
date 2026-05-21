// Package connections is the business layer around saved Cassandra connection
// configs. It owns validation, owner scoping, and the encrypt-on-write /
// decrypt-on-read seam.
package connections

import (
	"time"

	"github.com/ashikkabeer/cassandra-gui/internal/metastore"
)

// ConnectionDTO is the wire shape returned to clients. It deliberately omits
// `auth_password_enc` and `tls_client_key_enc` — secrets never leave the server.
type ConnectionDTO struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Hosts            []string `json:"hosts"`
	Port             int      `json:"port"`
	Datacenter       string   `json:"datacenter,omitempty"`
	DefaultKeyspace  string   `json:"default_keyspace,omitempty"`
	AuthUsername     string   `json:"auth_username,omitempty"`
	HasPassword      bool     `json:"has_password"`
	TLSEnabled       bool     `json:"tls_enabled"`
	TLSSkipVerify    bool     `json:"tls_skip_verify"`
	HasTLSCA         bool     `json:"has_tls_ca"`
	HasTLSClientCert bool     `json:"has_tls_client_cert"`
	ReadOnly         bool     `json:"read_only"`
	Consistency      string   `json:"consistency"`
	ConnectTimeoutMS int      `json:"connect_timeout_ms"`
	RequestTimeoutMS int      `json:"request_timeout_ms"`
	CreatedAt        string   `json:"created_at"`
	UpdatedAt        string   `json:"updated_at"`
	LastUsedAt       string   `json:"last_used_at,omitempty"`
}

func ToDTO(c *metastore.Connection) ConnectionDTO {
	d := ConnectionDTO{
		ID:               c.ID,
		Name:             c.Name,
		Hosts:            c.Hosts,
		Port:             c.Port,
		HasPassword:      len(c.AuthPasswordEnc) > 0,
		TLSEnabled:       c.TLSEnabled,
		TLSSkipVerify:    c.TLSSkipVerify,
		HasTLSCA:         c.TLSCACert.Valid && c.TLSCACert.String != "",
		HasTLSClientCert: c.TLSClientCert.Valid && c.TLSClientCert.String != "" && len(c.TLSClientKeyEnc) > 0,
		ReadOnly:         c.ReadOnly,
		Consistency:      c.Consistency,
		ConnectTimeoutMS: c.ConnectTimeoutMS,
		RequestTimeoutMS: c.RequestTimeoutMS,
		CreatedAt:        c.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:        c.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if c.Datacenter.Valid {
		d.Datacenter = c.Datacenter.String
	}
	if c.DefaultKeyspace.Valid {
		d.DefaultKeyspace = c.DefaultKeyspace.String
	}
	if c.AuthUsername.Valid {
		d.AuthUsername = c.AuthUsername.String
	}
	if c.LastUsedAt.Valid {
		d.LastUsedAt = c.LastUsedAt.Time.UTC().Format(time.RFC3339)
	}
	return d
}

// CreateRequest is the body of POST /connections (and POST /connections/test).
// Plaintext password / TLS client key arrive here once and never touch the DB
// unencrypted.
type CreateRequest struct {
	Name             string   `json:"name"`
	Hosts            []string `json:"hosts"`
	Port             int      `json:"port"`
	Datacenter       string   `json:"datacenter"`
	DefaultKeyspace  string   `json:"default_keyspace"`
	AuthUsername     string   `json:"auth_username"`
	Password         string   `json:"password"`
	TLSEnabled       bool     `json:"tls_enabled"`
	TLSSkipVerify    bool     `json:"tls_skip_verify"`
	TLSCACert        string   `json:"tls_ca_cert"`
	TLSClientCert    string   `json:"tls_client_cert"`
	TLSClientKey     string   `json:"tls_client_key"`
	ReadOnly         bool     `json:"read_only"`
	Consistency      string   `json:"consistency"`
	ConnectTimeoutMS int      `json:"connect_timeout_ms"`
	RequestTimeoutMS int      `json:"request_timeout_ms"`
}

// UpdateRequest mirrors CreateRequest but every field is a pointer — a nil
// pointer means "leave this field alone". For `Password` / `TLSClientKey`:
// nil → retain existing ciphertext, "" → clear, non-empty → re-encrypt.
type UpdateRequest struct {
	Name             *string   `json:"name,omitempty"`
	Hosts            *[]string `json:"hosts,omitempty"`
	Port             *int      `json:"port,omitempty"`
	Datacenter       *string   `json:"datacenter,omitempty"`
	DefaultKeyspace  *string   `json:"default_keyspace,omitempty"`
	AuthUsername     *string   `json:"auth_username,omitempty"`
	Password         *string   `json:"password,omitempty"`
	TLSEnabled       *bool     `json:"tls_enabled,omitempty"`
	TLSSkipVerify    *bool     `json:"tls_skip_verify,omitempty"`
	TLSCACert        *string   `json:"tls_ca_cert,omitempty"`
	TLSClientCert    *string   `json:"tls_client_cert,omitempty"`
	TLSClientKey     *string   `json:"tls_client_key,omitempty"`
	ReadOnly         *bool     `json:"read_only,omitempty"`
	Consistency      *string   `json:"consistency,omitempty"`
	ConnectTimeoutMS *int      `json:"connect_timeout_ms,omitempty"`
	RequestTimeoutMS *int      `json:"request_timeout_ms,omitempty"`
}
