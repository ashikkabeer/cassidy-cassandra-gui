package metastore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// QueryHistoryEntry is the persisted shape of one query execution.
type QueryHistoryEntry struct {
	ID            string
	UserID        string
	ConnectionID  sql.NullString
	Keyspace      sql.NullString
	CQL           string
	StatementKind string
	Success       bool
	ErrorCode     sql.NullString
	ErrorMessage  sql.NullString
	RowCount      int
	DurationMS    int
	ExecutedAt    time.Time
}

type QueryHistory struct{ db *sql.DB }

func NewQueryHistory(db *sql.DB) *QueryHistory { return &QueryHistory{db: db} }

var ErrQueryHistoryNotFound = errors.New("query history entry not found")

type CreateQueryHistoryParams struct {
	UserID        string
	ConnectionID  string // optional
	Keyspace      string // optional
	CQL           string
	StatementKind string
	Success       bool
	ErrorCode     string
	ErrorMessage  string
	RowCount      int
	DurationMS    int
}

// Create inserts a history row. Returns the generated id. Called fire-and-
// forget after every query so a slow SQLite write never blocks the user's
// result response.
func (h *QueryHistory) Create(ctx context.Context, p CreateQueryHistoryParams) (string, error) {
	if p.UserID == "" || p.CQL == "" || p.StatementKind == "" {
		return "", fmt.Errorf("user_id, cql, statement_kind are required")
	}
	id := uuid.NewString()
	_, err := h.db.ExecContext(ctx, `
		INSERT INTO query_history (
			id, user_id, connection_id, keyspace, cql, statement_kind,
			success, error_code, error_message, row_count, duration_ms, executed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, p.UserID,
		nullableString(p.ConnectionID), nullableString(p.Keyspace),
		p.CQL, p.StatementKind,
		boolToInt(p.Success),
		nullableString(p.ErrorCode), nullableString(p.ErrorMessage),
		p.RowCount, p.DurationMS,
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return "", fmt.Errorf("insert query_history: %w", err)
	}
	return id, nil
}

// ListByUserFilter narrows the result.
type ListByUserFilter struct {
	ConnectionID string // optional — empty = any
	Kind         string // optional — empty = any
	Success      *bool  // optional — nil = any
	Limit        int    // <=0 → default 100
	Before       string // optional — executed_at < this (RFC3339); for paging
}

func (h *QueryHistory) ListByUser(ctx context.Context, userID string, f ListByUserFilter) ([]QueryHistoryEntry, error) {
	q := `SELECT id, user_id, connection_id, keyspace, cql, statement_kind,
	             success, error_code, error_message, row_count, duration_ms, executed_at
	      FROM query_history WHERE user_id = ?`
	args := []any{userID}
	if f.ConnectionID != "" {
		q += " AND connection_id = ?"
		args = append(args, f.ConnectionID)
	}
	if f.Kind != "" {
		q += " AND statement_kind = ?"
		args = append(args, f.Kind)
	}
	if f.Success != nil {
		q += " AND success = ?"
		args = append(args, boolToInt(*f.Success))
	}
	if f.Before != "" {
		q += " AND executed_at < ?"
		args = append(args, f.Before)
	}
	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q += " ORDER BY datetime(executed_at) DESC LIMIT ?"
	args = append(args, limit)

	rows, err := h.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list query_history: %w", err)
	}
	defer rows.Close()

	var out []QueryHistoryEntry
	for rows.Next() {
		e, err := scanQueryHistory(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *e)
	}
	return out, rows.Err()
}

// Delete removes a single entry owned by userID. Returns ErrQueryHistoryNotFound
// if the entry doesn't exist or belongs to another user.
func (h *QueryHistory) Delete(ctx context.Context, userID, id string) error {
	res, err := h.db.ExecContext(ctx,
		`DELETE FROM query_history WHERE id = ? AND user_id = ?`, id, userID,
	)
	if err != nil {
		return fmt.Errorf("delete query_history: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrQueryHistoryNotFound
	}
	return nil
}

// Purge drops every history row for a user; used during account-delete paths.
func (h *QueryHistory) Purge(ctx context.Context, userID string) error {
	_, err := h.db.ExecContext(ctx, `DELETE FROM query_history WHERE user_id = ?`, userID)
	return err
}

func scanQueryHistory(row interface{ Scan(...any) error }) (*QueryHistoryEntry, error) {
	var e QueryHistoryEntry
	var success int
	var executedAt string
	err := row.Scan(
		&e.ID, &e.UserID, &e.ConnectionID, &e.Keyspace, &e.CQL, &e.StatementKind,
		&success, &e.ErrorCode, &e.ErrorMessage, &e.RowCount, &e.DurationMS,
		&executedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrQueryHistoryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan query_history: %w", err)
	}
	e.Success = success == 1
	e.ExecutedAt = parseTime(executedAt)
	return &e, nil
}

// trim helper for filters
func trimToLower(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
