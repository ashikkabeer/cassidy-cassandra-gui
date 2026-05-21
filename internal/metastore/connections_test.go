package metastore

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
)

func openConnTestDB(t *testing.T) (*Users, *Connections) {
	t.Helper()
	dir := t.TempDir()
	db, err := Open(context.Background(), filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewUsers(db), NewConnections(db)
}

func seedUser(t *testing.T, users *Users, name string) *User {
	t.Helper()
	u, err := users.Create(context.Background(), CreateUserParams{
		Username:     name,
		PasswordHash: "$argon2id$v=19$m=65536,t=3,p=2$aaaa$bbbb",
		Role:         RoleEditor,
	})
	if err != nil {
		t.Fatalf("seed user %q: %v", name, err)
	}
	return u
}

func TestConnectionRoundTrip(t *testing.T) {
	users, conns := openConnTestDB(t)
	ctx := context.Background()
	owner := seedUser(t, users, "alice")

	encPw := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	c, err := conns.Create(ctx, CreateConnectionParams{
		OwnerID:          owner.ID,
		Name:             "prod-eu",
		Hosts:            []string{"10.0.0.1", "10.0.0.2"},
		Port:             9042,
		Datacenter:       "eu-west",
		AuthUsername:     "ro",
		AuthPasswordEnc:  encPw,
		Consistency:      "LOCAL_QUORUM",
		ConnectTimeoutMS: 5000,
		RequestTimeoutMS: 15000,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if c.Name != "prod-eu" || len(c.Hosts) != 2 || !bytes.Equal(c.AuthPasswordEnc, encPw) {
		t.Fatalf("round trip wrong: %+v", c)
	}

	got, err := conns.GetForOwner(ctx, owner.ID, c.ID)
	if err != nil {
		t.Fatalf("get for owner: %v", err)
	}
	if got.ID != c.ID || !bytes.Equal(got.AuthPasswordEnc, encPw) {
		t.Fatalf("get for owner returned wrong data")
	}
}

func TestConnectionOwnerScoped(t *testing.T) {
	users, conns := openConnTestDB(t)
	ctx := context.Background()
	alice := seedUser(t, users, "alice")
	bob := seedUser(t, users, "bob")

	c, err := conns.Create(ctx, CreateConnectionParams{
		OwnerID: alice.ID, Name: "alice-conn", Hosts: []string{"1.1.1.1"}, Port: 9042,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := conns.GetForOwner(ctx, bob.ID, c.ID); err == nil {
		t.Fatal("bob should not be able to fetch alice's connection")
	}
	list, err := conns.ListByOwner(ctx, bob.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("bob should see no connections, got %d", len(list))
	}
}

func TestConnectionUpdateRetainsSecretOnNil(t *testing.T) {
	users, conns := openConnTestDB(t)
	ctx := context.Background()
	owner := seedUser(t, users, "alice")

	encPw := []byte{1, 2, 3, 4, 5}
	c, _ := conns.Create(ctx, CreateConnectionParams{
		OwnerID: owner.ID, Name: "c1", Hosts: []string{"x"}, Port: 9042,
		AuthPasswordEnc: encPw,
	})

	name := "c1-renamed"
	// Pointer to nil bytes means "retain". Pointer to []byte{} means "clear".
	if err := conns.Update(ctx, owner.ID, c.ID, UpdateConnectionParams{Name: &name}); err != nil {
		t.Fatalf("update name only: %v", err)
	}
	got, _ := conns.GetForOwner(ctx, owner.ID, c.ID)
	if got.Name != "c1-renamed" {
		t.Fatalf("name not updated: %q", got.Name)
	}
	if !bytes.Equal(got.AuthPasswordEnc, encPw) {
		t.Fatalf("password should have been retained, was %v", got.AuthPasswordEnc)
	}

	// Now clear the password (pointer to empty slice).
	empty := []byte{}
	if err := conns.Update(ctx, owner.ID, c.ID, UpdateConnectionParams{AuthPasswordEnc: &empty}); err != nil {
		t.Fatalf("clear password: %v", err)
	}
	got, _ = conns.GetForOwner(ctx, owner.ID, c.ID)
	if len(got.AuthPasswordEnc) != 0 {
		t.Fatalf("password should be cleared, got %v", got.AuthPasswordEnc)
	}
}

func TestConnectionDelete(t *testing.T) {
	users, conns := openConnTestDB(t)
	ctx := context.Background()
	alice := seedUser(t, users, "alice")
	bob := seedUser(t, users, "bob")

	c, _ := conns.Create(ctx, CreateConnectionParams{
		OwnerID: alice.ID, Name: "c1", Hosts: []string{"x"}, Port: 9042,
	})

	// Bob can't delete Alice's connection.
	if err := conns.Delete(ctx, bob.ID, c.ID); err == nil {
		t.Fatal("expected ErrConnectionNotFound when non-owner deletes")
	}
	// Alice can.
	if err := conns.Delete(ctx, alice.ID, c.ID); err != nil {
		t.Fatalf("alice delete: %v", err)
	}
	if _, err := conns.GetForOwner(ctx, alice.ID, c.ID); err == nil {
		t.Fatal("connection should be gone")
	}
}
