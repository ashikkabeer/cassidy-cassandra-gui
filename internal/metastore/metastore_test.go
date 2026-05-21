package metastore

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func openTestDB(t *testing.T) (*Users, *Sessions) {
	t.Helper()
	dir := t.TempDir()
	db, err := Open(context.Background(), filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewUsers(db), NewSessions(db)
}

func TestMigrationsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	for i := 0; i < 3; i++ {
		db, err := Open(context.Background(), path)
		if err != nil {
			t.Fatalf("open iter %d: %v", i, err)
		}
		_ = db.Close()
	}
}

func TestUsersCRUD(t *testing.T) {
	users, _ := openTestDB(t)
	ctx := context.Background()

	u, err := users.Create(ctx, CreateUserParams{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "$argon2id$v=19$m=65536,t=3,p=2$aaaa$bbbb",
		Role:         RoleEditor,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if u.Username != "alice" {
		t.Fatalf("username: %q", u.Username)
	}

	got, err := users.GetByUsername(ctx, "alice")
	if err != nil {
		t.Fatalf("get by username: %v", err)
	}
	if got.ID != u.ID {
		t.Fatal("mismatched id")
	}

	role := RoleAdmin
	if err := users.Update(ctx, u.ID, UpdateUserParams{Role: &role}); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ = users.GetByID(ctx, u.ID)
	if got.Role != RoleAdmin {
		t.Fatalf("role not updated: %q", got.Role)
	}
}

func TestFirstAdminGuard(t *testing.T) {
	users, _ := openTestDB(t)
	ctx := context.Background()
	_, ok, err := users.CreateFirstAdmin(ctx, CreateUserParams{
		Username: "root", PasswordHash: "$argon2id$v=19$m=65536,t=3,p=2$aaaa$bbbb",
		Role: RoleAdmin,
	})
	if err != nil || !ok {
		t.Fatalf("first admin create: ok=%v err=%v", ok, err)
	}
	// Second attempt must report ok=false.
	_, ok, err = users.CreateFirstAdmin(ctx, CreateUserParams{
		Username: "root2", PasswordHash: "$argon2id$v=19$m=65536,t=3,p=2$cccc$dddd",
		Role: RoleAdmin,
	})
	if err != nil {
		t.Fatalf("second admin err: %v", err)
	}
	if ok {
		t.Fatal("CreateFirstAdmin should not insert a second admin")
	}
}

func TestSessionExpiry(t *testing.T) {
	users, sessions := openTestDB(t)
	ctx := context.Background()
	u, err := users.Create(ctx, CreateUserParams{
		Username: "alice", PasswordHash: "$argon2id$v=19$m=65536,t=3,p=2$aaaa$bbbb",
		Role: RoleViewer,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	expired := time.Now().Add(-1 * time.Minute)
	if err := sessions.Create(ctx, CreateSessionParams{
		ID: "expired-tok", UserID: u.ID, ExpiresAt: expired,
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if _, err := sessions.Get(ctx, "expired-tok"); err == nil {
		t.Fatal("expected Get to reject expired session")
	}

	fresh := time.Now().Add(1 * time.Hour)
	if err := sessions.Create(ctx, CreateSessionParams{
		ID: "fresh-tok", UserID: u.ID, ExpiresAt: fresh,
	}); err != nil {
		t.Fatalf("create fresh: %v", err)
	}
	got, err := sessions.Get(ctx, "fresh-tok")
	if err != nil {
		t.Fatalf("get fresh: %v", err)
	}
	if got.UserID != u.ID {
		t.Fatal("mismatched user")
	}

	n, err := sessions.DeleteExpired(ctx)
	if err != nil {
		t.Fatalf("delete expired: %v", err)
	}
	if n != 0 {
		// The expired session was already deleted lazily by Get().
		t.Logf("DeleteExpired removed %d (lazy delete in Get likely already ran)", n)
	}
}
