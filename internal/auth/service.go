// Package auth implements Cassidy's local-account authentication: password
// hashing, server-side sessions, CSRF, rate-limiting, and first-run setup.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ashikkabeer/cassandra-gui/internal/crypto"
	"github.com/ashikkabeer/cassandra-gui/internal/metastore"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInactiveAccount    = errors.New("account inactive")
	ErrPasswordTooShort   = errors.New("password too short (min 12 chars)")
	ErrCurrentPwWrong     = errors.New("current password is wrong")
	ErrSetupClosed        = errors.New("setup already complete")
	ErrInvalidSetupToken  = errors.New("invalid setup token")
)

const minPasswordLen = 12

// Service composes user + session repos with crypto operations into a small
// transactional surface used by HTTP handlers.
type Service struct {
	users      *metastore.Users
	sessions   *metastore.Sessions
	setup      *SetupToken
	sessionTTL time.Duration
}

func NewService(users *metastore.Users, sessions *metastore.Sessions, setup *SetupToken, sessionTTL time.Duration) *Service {
	return &Service{users: users, sessions: sessions, setup: setup, sessionTTL: sessionTTL}
}

// Login verifies username + password and creates a new session row.
// Errors are intentionally generic (ErrInvalidCredentials) to avoid leaking
// whether the user exists.
func (s *Service) Login(ctx context.Context, username, password, ip, ua string) (*metastore.User, *metastore.Session, string, error) {
	u, err := s.users.GetByUsername(ctx, strings.TrimSpace(username))
	if errors.Is(err, metastore.ErrUserNotFound) {
		// Constant-time-ish: still do a hash op so timing leaks less info.
		_, _ = crypto.VerifyPassword(password, dummyHash)
		return nil, nil, "", ErrInvalidCredentials
	}
	if err != nil {
		return nil, nil, "", err
	}
	if !u.IsActive {
		return nil, nil, "", ErrInactiveAccount
	}
	ok, err := crypto.VerifyPassword(password, u.PasswordHash)
	if err != nil || !ok {
		return nil, nil, "", ErrInvalidCredentials
	}
	token, sess, err := s.issueSession(ctx, u.ID, ip, ua)
	if err != nil {
		return nil, nil, "", err
	}
	return u, sess, token, nil
}

// Logout deletes the session row.
func (s *Service) Logout(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return nil
	}
	return s.sessions.Delete(ctx, sessionID)
}

// Setup consumes the first-run setup token and creates the first admin.
func (s *Service) Setup(ctx context.Context, suppliedToken, username, email, password, ip, ua string) (*metastore.User, *metastore.Session, string, error) {
	if err := validatePassword(password); err != nil {
		return nil, nil, "", err
	}
	if !s.setup.Verify(suppliedToken) {
		return nil, nil, "", ErrInvalidSetupToken
	}
	hash, err := crypto.HashPassword(password)
	if err != nil {
		return nil, nil, "", fmt.Errorf("hash password: %w", err)
	}
	u, ok, err := s.users.CreateFirstAdmin(ctx, metastore.CreateUserParams{
		Username:     strings.TrimSpace(username),
		Email:        strings.TrimSpace(email),
		PasswordHash: hash,
		Role:         metastore.RoleAdmin,
	})
	if err != nil {
		return nil, nil, "", err
	}
	if !ok {
		return nil, nil, "", ErrSetupClosed
	}
	// Consume the token.
	s.setup.Consume()

	token, sess, err := s.issueSession(ctx, u.ID, ip, ua)
	if err != nil {
		return nil, nil, "", err
	}
	return u, sess, token, nil
}

// ChangePassword verifies the current password and writes a new hash.
func (s *Service) ChangePassword(ctx context.Context, userID, current, next string) error {
	if err := validatePassword(next); err != nil {
		return err
	}
	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	ok, err := crypto.VerifyPassword(current, u.PasswordHash)
	if err != nil || !ok {
		return ErrCurrentPwWrong
	}
	hash, err := crypto.HashPassword(next)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	return s.users.SetPasswordHash(ctx, userID, hash, false)
}

// ResetPassword sets a server-generated temporary password and flags must_reset_pw.
// Returned plaintext should only be displayed once to the admin caller.
func (s *Service) ResetPassword(ctx context.Context, userID string) (string, error) {
	temp, err := randomToken(12)
	if err != nil {
		return "", err
	}
	// 12 base64url chars ≈ 72 bits entropy. Concatenate to clear the min-length bar.
	temp = "Cs!" + temp
	hash, err := crypto.HashPassword(temp)
	if err != nil {
		return "", fmt.Errorf("hash temp password: %w", err)
	}
	if err := s.users.SetPasswordHash(ctx, userID, hash, true); err != nil {
		return "", err
	}
	// Invalidate any existing sessions for the user.
	_ = s.sessions.DeleteByUser(ctx, userID)
	return temp, nil
}

// CurrentUser returns the user owning a session, if any. The session is touched
// on the way through.
func (s *Service) CurrentUser(ctx context.Context, sessionID string) (*metastore.User, *metastore.Session, error) {
	if sessionID == "" {
		return nil, nil, ErrInvalidCredentials
	}
	sess, err := s.sessions.Get(ctx, sessionID)
	if err != nil {
		return nil, nil, err
	}
	u, err := s.users.GetByID(ctx, sess.UserID)
	if err != nil {
		return nil, nil, err
	}
	if !u.IsActive {
		return nil, nil, ErrInactiveAccount
	}
	s.sessions.Touch(ctx, sessionID)
	return u, sess, nil
}

func (s *Service) issueSession(ctx context.Context, userID, ip, ua string) (string, *metastore.Session, error) {
	token, err := randomToken(32)
	if err != nil {
		return "", nil, err
	}
	expires := time.Now().Add(s.sessionTTL)
	if err := s.sessions.Create(ctx, metastore.CreateSessionParams{
		ID:        token,
		UserID:    userID,
		ExpiresAt: expires,
		IP:        ip,
		UserAgent: ua,
	}); err != nil {
		return "", nil, err
	}
	sess := &metastore.Session{ID: token, UserID: userID, ExpiresAt: expires, CreatedAt: time.Now()}
	return token, sess, nil
}

func validatePassword(pw string) error {
	if len(pw) < minPasswordLen {
		return ErrPasswordTooShort
	}
	return nil
}

func randomToken(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("read random: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// dummyHash is a precomputed Argon2id hash used during failed username lookups
// so the wall-clock cost of "no such user" approximates "wrong password" —
// limiting username-enumeration via timing. Generated once at process start.
var dummyHash string

func init() {
	h, err := crypto.HashPassword("not-a-real-password-just-balancing-timing-x9F!")
	if err == nil {
		dummyHash = h
	}
}
