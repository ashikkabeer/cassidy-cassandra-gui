package auth

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/ashikkabeer/cassandra-gui/internal/crypto"
	"github.com/ashikkabeer/cassandra-gui/internal/metastore"
)

func newTestService(t *testing.T) (*Service, *metastore.Users, *metastore.Sessions, *SetupToken) {
	t.Helper()
	dir := t.TempDir()
	db, err := metastore.Open(context.Background(), filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	users := metastore.NewUsers(db)
	sessions := metastore.NewSessions(db)
	tok, err := LoadOrCreateSetupToken(true, "cs_setup_test_token", filepath.Join(dir, "setup.txt"))
	if err != nil {
		t.Fatalf("setup token: %v", err)
	}
	svc := NewService(users, sessions, tok, 24*time.Hour)
	return svc, users, sessions, tok
}

func TestLoginHappyPath(t *testing.T) {
	svc, users, _, _ := newTestService(t)
	ctx := context.Background()

	hash, _ := crypto.HashPassword("hunter2-correctly-long")
	_, err := users.Create(ctx, metastore.CreateUserParams{
		Username:     "alice",
		PasswordHash: hash,
		Role:         metastore.RoleAdmin,
	})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	u, sess, tok, err := svc.Login(ctx, "alice", "hunter2-correctly-long", "127.0.0.1", "test-ua")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if u.Username != "alice" {
		t.Fatalf("username: %q", u.Username)
	}
	if tok == "" || sess.ID != tok {
		t.Fatal("session token mismatch")
	}
}

func TestLoginWrongPassword(t *testing.T) {
	svc, users, _, _ := newTestService(t)
	ctx := context.Background()
	hash, _ := crypto.HashPassword("right-password-12345")
	_, _ = users.Create(ctx, metastore.CreateUserParams{
		Username: "alice", PasswordHash: hash, Role: metastore.RoleViewer,
	})
	_, _, _, err := svc.Login(ctx, "alice", "wrong-password", "", "")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLoginUnknownUser(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	_, _, _, err := svc.Login(context.Background(), "ghost", "doesnt-matter", "", "")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLoginInactiveAccount(t *testing.T) {
	svc, users, _, _ := newTestService(t)
	ctx := context.Background()
	hash, _ := crypto.HashPassword("right-password-12345")
	u, _ := users.Create(ctx, metastore.CreateUserParams{
		Username: "alice", PasswordHash: hash, Role: metastore.RoleViewer,
	})
	inactive := false
	_ = users.Update(ctx, u.ID, metastore.UpdateUserParams{IsActive: &inactive})
	_, _, _, err := svc.Login(ctx, "alice", "right-password-12345", "", "")
	if !errors.Is(err, ErrInactiveAccount) {
		t.Fatalf("expected ErrInactiveAccount, got %v", err)
	}
}

func TestSetupHappyPath(t *testing.T) {
	svc, _, _, tok := newTestService(t)
	ctx := context.Background()
	if !tok.Open() {
		t.Fatal("setup token should be open")
	}
	u, _, _, err := svc.Setup(ctx, "cs_setup_test_token", "root", "root@example.com", "this-is-long-enough", "", "")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	if u.Role != metastore.RoleAdmin {
		t.Fatalf("expected admin role, got %q", u.Role)
	}
	if tok.Open() {
		t.Fatal("setup token should be consumed")
	}
	// Re-attempt must fail.
	_, _, _, err = svc.Setup(ctx, "cs_setup_test_token", "root2", "", "this-is-long-enough", "", "")
	if !errors.Is(err, ErrInvalidSetupToken) {
		t.Fatalf("expected ErrInvalidSetupToken on re-use, got %v", err)
	}
}

func TestSetupBadToken(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	_, _, _, err := svc.Setup(context.Background(), "wrong-token", "root", "", "this-is-long-enough", "", "")
	if !errors.Is(err, ErrInvalidSetupToken) {
		t.Fatalf("expected ErrInvalidSetupToken, got %v", err)
	}
}

func TestChangePassword(t *testing.T) {
	svc, users, _, _ := newTestService(t)
	ctx := context.Background()
	hash, _ := crypto.HashPassword("first-password-1234")
	u, _ := users.Create(ctx, metastore.CreateUserParams{
		Username: "alice", PasswordHash: hash, Role: metastore.RoleViewer,
	})
	if err := svc.ChangePassword(ctx, u.ID, "first-password-1234", "second-password-99"); err != nil {
		t.Fatalf("change pw: %v", err)
	}
	// Old password should now fail.
	_, _, _, err := svc.Login(ctx, "alice", "first-password-1234", "", "")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("old password still works: %v", err)
	}
	if _, _, _, err := svc.Login(ctx, "alice", "second-password-99", "", ""); err != nil {
		t.Fatalf("new password failed: %v", err)
	}
}

func TestChangePasswordWrongCurrent(t *testing.T) {
	svc, users, _, _ := newTestService(t)
	ctx := context.Background()
	hash, _ := crypto.HashPassword("first-password-1234")
	u, _ := users.Create(ctx, metastore.CreateUserParams{
		Username: "alice", PasswordHash: hash, Role: metastore.RoleViewer,
	})
	err := svc.ChangePassword(ctx, u.ID, "wrong", "another-long-password")
	if !errors.Is(err, ErrCurrentPwWrong) {
		t.Fatalf("expected ErrCurrentPwWrong, got %v", err)
	}
}

func TestChangePasswordTooShort(t *testing.T) {
	svc, users, _, _ := newTestService(t)
	ctx := context.Background()
	hash, _ := crypto.HashPassword("first-password-1234")
	u, _ := users.Create(ctx, metastore.CreateUserParams{
		Username: "alice", PasswordHash: hash, Role: metastore.RoleViewer,
	})
	err := svc.ChangePassword(ctx, u.ID, "first-password-1234", "short")
	if !errors.Is(err, ErrPasswordTooShort) {
		t.Fatalf("expected ErrPasswordTooShort, got %v", err)
	}
}

func TestResetPasswordRevokesSessions(t *testing.T) {
	svc, users, sessions, _ := newTestService(t)
	ctx := context.Background()
	hash, _ := crypto.HashPassword("first-password-1234")
	u, _ := users.Create(ctx, metastore.CreateUserParams{
		Username: "alice", PasswordHash: hash, Role: metastore.RoleViewer,
	})
	if _, _, _, err := svc.Login(ctx, "alice", "first-password-1234", "", ""); err != nil {
		t.Fatalf("login: %v", err)
	}
	sessList, _ := sessions.ListByUser(ctx, u.ID)
	if len(sessList) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessList))
	}
	if _, err := svc.ResetPassword(ctx, u.ID); err != nil {
		t.Fatalf("reset: %v", err)
	}
	sessList, _ = sessions.ListByUser(ctx, u.ID)
	if len(sessList) != 0 {
		t.Fatalf("reset should revoke sessions; %d remain", len(sessList))
	}
}

func TestRateLimiter(t *testing.T) {
	l := NewIPRateLimiter(2, 1*time.Second)
	// Two should pass for the same IP, third should be blocked immediately.
	if !l.get("1.1.1.1").Allow() || !l.get("1.1.1.1").Allow() {
		t.Fatal("first two should pass")
	}
	if l.get("1.1.1.1").Allow() {
		t.Fatal("third should be blocked")
	}
	// Different IP has its own bucket.
	if !l.get("2.2.2.2").Allow() {
		t.Fatal("different IP should pass")
	}
}
