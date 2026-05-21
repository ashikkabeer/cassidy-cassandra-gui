package metastore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Session struct {
	ID         string
	UserID     string
	CreatedAt  time.Time
	LastSeenAt time.Time
	ExpiresAt  time.Time
	IPAddress  sql.NullString
	UserAgent  sql.NullString
}

type Sessions struct{ db *sql.DB }

func NewSessions(db *sql.DB) *Sessions { return &Sessions{db: db} }

var ErrSessionNotFound = errors.New("session not found")

type CreateSessionParams struct {
	ID        string // opaque token (becomes cookie value)
	UserID    string
	ExpiresAt time.Time
	IP        string
	UserAgent string
}

func (s *Sessions) Create(ctx context.Context, p CreateSessionParams) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO app_sessions
			(id, user_id, created_at, last_seen_at, expires_at, ip_address, user_agent)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.UserID, now, now,
		p.ExpiresAt.UTC().Format(time.RFC3339Nano),
		nullableString(p.IP), nullableString(p.UserAgent),
	)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

// Get returns the session iff it has not expired. Touches last_seen_at.
func (s *Sessions) Get(ctx context.Context, id string) (*Session, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, created_at, last_seen_at, expires_at, ip_address, user_agent
		FROM app_sessions WHERE id = ?`, id)
	out, err := scanSession(row)
	if err != nil {
		return nil, err
	}
	if time.Now().After(out.ExpiresAt) {
		_ = s.Delete(ctx, id)
		return nil, ErrSessionNotFound
	}
	return out, nil
}

// Touch updates last_seen_at on every authenticated request.
func (s *Sessions) Touch(ctx context.Context, id string) {
	_, _ = s.db.ExecContext(ctx,
		`UPDATE app_sessions SET last_seen_at = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339Nano), id,
	)
}

func (s *Sessions) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM app_sessions WHERE id = ?`, id)
	return err
}

func (s *Sessions) DeleteByUser(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM app_sessions WHERE user_id = ?`, userID)
	return err
}

func (s *Sessions) DeleteOthersForUser(ctx context.Context, userID, keepID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM app_sessions WHERE user_id = ? AND id <> ?`, userID, keepID)
	return err
}

func (s *Sessions) DeleteExpired(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM app_sessions WHERE expires_at < ?`,
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Sessions) ListByUser(ctx context.Context, userID string) ([]Session, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, created_at, last_seen_at, expires_at, ip_address, user_agent
		FROM app_sessions WHERE user_id = ? AND expires_at >= ?
		ORDER BY last_seen_at DESC`,
		userID, time.Now().UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		sess, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *sess)
	}
	return out, rows.Err()
}

func scanSession(row interface{ Scan(...any) error }) (*Session, error) {
	var s Session
	var createdAt, lastSeenAt, expiresAt string
	err := row.Scan(
		&s.ID, &s.UserID, &createdAt, &lastSeenAt, &expiresAt,
		&s.IPAddress, &s.UserAgent,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan session: %w", err)
	}
	s.CreatedAt = parseTime(createdAt)
	s.LastSeenAt = parseTime(lastSeenAt)
	s.ExpiresAt = parseTime(expiresAt)
	return &s, nil
}

func nullableString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
