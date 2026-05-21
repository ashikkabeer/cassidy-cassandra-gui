package metastore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleAdmin  Role = "admin"
	RoleEditor Role = "editor"
	RoleViewer Role = "viewer"
)

func (r Role) IsValid() bool {
	switch r {
	case RoleAdmin, RoleEditor, RoleViewer:
		return true
	}
	return false
}

type User struct {
	ID           string
	Username     string
	Email        sql.NullString
	PasswordHash string
	Role         Role
	IsActive     bool
	MustResetPW  bool
	CreatedBy    sql.NullString
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Users struct{ db *sql.DB }

func NewUsers(db *sql.DB) *Users { return &Users{db: db} }

var ErrUserNotFound = errors.New("user not found")

type CreateUserParams struct {
	Username     string
	Email        string
	PasswordHash string
	Role         Role
	MustResetPW  bool
	CreatedByID  string // optional
}

func (u *Users) Create(ctx context.Context, p CreateUserParams) (*User, error) {
	if !p.Role.IsValid() {
		return nil, fmt.Errorf("invalid role %q", p.Role)
	}
	id := uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var createdBy sql.NullString
	if p.CreatedByID != "" {
		createdBy = sql.NullString{String: p.CreatedByID, Valid: true}
	}
	var email sql.NullString
	if p.Email != "" {
		email = sql.NullString{String: p.Email, Valid: true}
	}
	_, err := u.db.ExecContext(ctx,
		`INSERT INTO app_users
			(id, username, email, password_hash, role, is_active, must_reset_pw, created_by, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, 1, ?, ?, ?, ?)`,
		id, p.Username, email, p.PasswordHash, string(p.Role), boolToInt(p.MustResetPW), createdBy, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}
	return u.GetByID(ctx, id)
}

// CreateFirstAdmin atomically creates the first admin user, but only if no admin
// currently exists. Returns nil, false if an admin already exists (caller should
// reject the setup attempt). Used to guard /auth/setup against races.
func (u *Users) CreateFirstAdmin(ctx context.Context, p CreateUserParams) (*User, bool, error) {
	if p.Role != RoleAdmin {
		return nil, false, fmt.Errorf("CreateFirstAdmin requires role=admin")
	}
	id := uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var email sql.NullString
	if p.Email != "" {
		email = sql.NullString{String: p.Email, Valid: true}
	}
	res, err := u.db.ExecContext(ctx,
		`INSERT INTO app_users
			(id, username, email, password_hash, role, is_active, must_reset_pw, created_at, updated_at)
		 SELECT ?, ?, ?, ?, 'admin', 1, 0, ?, ?
		 WHERE NOT EXISTS (
		   SELECT 1 FROM app_users WHERE role='admin' AND is_active=1
		 )`,
		id, p.Username, email, p.PasswordHash, now, now,
	)
	if err != nil {
		return nil, false, fmt.Errorf("insert first admin: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, false, fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return nil, false, nil
	}
	user, err := u.GetByID(ctx, id)
	if err != nil {
		return nil, false, err
	}
	return user, true, nil
}

func (u *Users) GetByID(ctx context.Context, id string) (*User, error) {
	return u.queryOne(ctx, `WHERE id = ?`, id)
}

func (u *Users) GetByUsername(ctx context.Context, username string) (*User, error) {
	return u.queryOne(ctx, `WHERE username = ?`, username)
}

func (u *Users) queryOne(ctx context.Context, where string, args ...any) (*User, error) {
	row := u.db.QueryRowContext(ctx, `
		SELECT id, username, email, password_hash, role, is_active, must_reset_pw,
		       created_by, created_at, updated_at
		FROM app_users `+where, args...,
	)
	return scanUser(row)
}

func scanUser(row interface{ Scan(...any) error }) (*User, error) {
	var u User
	var role string
	var active, mustReset int
	var createdAt, updatedAt string
	err := row.Scan(
		&u.ID, &u.Username, &u.Email, &u.PasswordHash, &role,
		&active, &mustReset, &u.CreatedBy, &createdAt, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan user: %w", err)
	}
	u.Role = Role(role)
	u.IsActive = active == 1
	u.MustResetPW = mustReset == 1
	u.CreatedAt = parseTime(createdAt)
	u.UpdatedAt = parseTime(updatedAt)
	return &u, nil
}

func (u *Users) List(ctx context.Context) ([]User, error) {
	rows, err := u.db.QueryContext(ctx, `
		SELECT id, username, email, password_hash, role, is_active, must_reset_pw,
		       created_by, created_at, updated_at
		FROM app_users ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *u)
	}
	return out, rows.Err()
}

func (u *Users) CountAdmins(ctx context.Context) (int, error) {
	var n int
	err := u.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM app_users WHERE role='admin' AND is_active=1`,
	).Scan(&n)
	return n, err
}

type UpdateUserParams struct {
	Email    *string
	Role     *Role
	IsActive *bool
}

func (u *Users) Update(ctx context.Context, id string, p UpdateUserParams) error {
	sets := []string{}
	args := []any{}
	if p.Email != nil {
		sets = append(sets, "email = ?")
		args = append(args, *p.Email)
	}
	if p.Role != nil {
		if !p.Role.IsValid() {
			return fmt.Errorf("invalid role %q", *p.Role)
		}
		sets = append(sets, "role = ?")
		args = append(args, string(*p.Role))
	}
	if p.IsActive != nil {
		sets = append(sets, "is_active = ?")
		args = append(args, boolToInt(*p.IsActive))
	}
	if len(sets) == 0 {
		return nil
	}
	sets = append(sets, "updated_at = ?")
	args = append(args, time.Now().UTC().Format(time.RFC3339Nano))
	args = append(args, id)
	q := "UPDATE app_users SET " + joinComma(sets) + " WHERE id = ?"
	res, err := u.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (u *Users) SetPasswordHash(ctx context.Context, id, hash string, mustReset bool) error {
	res, err := u.db.ExecContext(ctx,
		`UPDATE app_users SET password_hash=?, must_reset_pw=?, updated_at=? WHERE id=?`,
		hash, boolToInt(mustReset), time.Now().UTC().Format(time.RFC3339Nano), id,
	)
	if err != nil {
		return fmt.Errorf("set password: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrUserNotFound
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func joinComma(xs []string) string {
	out := ""
	for i, s := range xs {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}

func parseTime(s string) time.Time {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
