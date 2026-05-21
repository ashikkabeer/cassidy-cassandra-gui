package metastore

import (
	"context"
	"path/filepath"
	"testing"
)

func openHistTestDB(t *testing.T) (*Users, *Connections, *QueryHistory) {
	t.Helper()
	dir := t.TempDir()
	db, err := Open(context.Background(), filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewUsers(db), NewConnections(db), NewQueryHistory(db)
}

func TestQueryHistoryRoundTrip(t *testing.T) {
	users, _, hist := openHistTestDB(t)
	ctx := context.Background()
	owner := seedUser(t, users, "alice")

	id, err := hist.Create(ctx, CreateQueryHistoryParams{
		UserID:        owner.ID,
		CQL:           "SELECT 1",
		StatementKind: "select",
		Success:       true,
		RowCount:      1,
		DurationMS:    42,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty id")
	}

	list, err := hist.ListByUser(ctx, owner.ID, ListByUserFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].ID != id || list[0].CQL != "SELECT 1" || !list[0].Success || list[0].DurationMS != 42 {
		t.Fatalf("round trip wrong: %+v", list)
	}
}

func TestQueryHistoryOwnerScoped(t *testing.T) {
	users, _, hist := openHistTestDB(t)
	ctx := context.Background()
	alice := seedUser(t, users, "alice")
	bob := seedUser(t, users, "bob")

	_, _ = hist.Create(ctx, CreateQueryHistoryParams{
		UserID: alice.ID, CQL: "SELECT 1", StatementKind: "select", Success: true,
	})
	list, _ := hist.ListByUser(ctx, bob.ID, ListByUserFilter{})
	if len(list) != 0 {
		t.Fatalf("bob should see no entries, got %d", len(list))
	}
}

func TestQueryHistoryFilters(t *testing.T) {
	users, _, hist := openHistTestDB(t)
	ctx := context.Background()
	u := seedUser(t, users, "alice")

	ok := true
	notOK := false
	_, _ = hist.Create(ctx, CreateQueryHistoryParams{UserID: u.ID, CQL: "SELECT 1", StatementKind: "select", Success: true})
	_, _ = hist.Create(ctx, CreateQueryHistoryParams{UserID: u.ID, CQL: "INSERT …", StatementKind: "insert", Success: true})
	_, _ = hist.Create(ctx, CreateQueryHistoryParams{UserID: u.ID, CQL: "DROP …", StatementKind: "drop", Success: false, ErrorCode: "cql_error"})

	got, _ := hist.ListByUser(ctx, u.ID, ListByUserFilter{Kind: "insert"})
	if len(got) != 1 || got[0].StatementKind != "insert" {
		t.Errorf("kind filter: %+v", got)
	}
	got, _ = hist.ListByUser(ctx, u.ID, ListByUserFilter{Success: &notOK})
	if len(got) != 1 || got[0].Success {
		t.Errorf("success=false filter: %+v", got)
	}
	got, _ = hist.ListByUser(ctx, u.ID, ListByUserFilter{Success: &ok})
	if len(got) != 2 {
		t.Errorf("success=true filter: %d", len(got))
	}
}

func TestQueryHistoryCascadeOnUserDelete(t *testing.T) {
	users, _, hist := openHistTestDB(t)
	ctx := context.Background()
	u := seedUser(t, users, "alice")
	_, _ = hist.Create(ctx, CreateQueryHistoryParams{UserID: u.ID, CQL: "x", StatementKind: "select", Success: true})

	// Hard-delete the user row (the app's normal flow soft-deletes via
	// is_active, but the FK is wired ON DELETE CASCADE for accidental hard-
	// deletes / future flows).
	if _, err := users.db.ExecContext(ctx, `DELETE FROM app_users WHERE id = ?`, u.ID); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	got, _ := hist.ListByUser(ctx, u.ID, ListByUserFilter{})
	if len(got) != 0 {
		t.Fatalf("expected cascade to drop history rows, got %d", len(got))
	}
}

func TestQueryHistoryConnSetNull(t *testing.T) {
	users, conns, hist := openHistTestDB(t)
	ctx := context.Background()
	u := seedUser(t, users, "alice")
	c, err := conns.Create(ctx, CreateConnectionParams{
		OwnerID: u.ID, Name: "c1", Hosts: []string{"x"}, Port: 9042, Consistency: "ONE",
	})
	if err != nil {
		t.Fatalf("create conn: %v", err)
	}
	id, _ := hist.Create(ctx, CreateQueryHistoryParams{
		UserID: u.ID, ConnectionID: c.ID, CQL: "x", StatementKind: "select", Success: true,
	})

	if err := conns.Delete(ctx, u.ID, c.ID); err != nil {
		t.Fatalf("delete conn: %v", err)
	}
	list, _ := hist.ListByUser(ctx, u.ID, ListByUserFilter{})
	if len(list) != 1 || list[0].ID != id {
		t.Fatalf("history should remain after conn delete: %+v", list)
	}
	if list[0].ConnectionID.Valid {
		t.Fatalf("expected connection_id to be NULL after conn delete, got %v", list[0].ConnectionID)
	}
}

func TestQueryHistoryDelete(t *testing.T) {
	users, _, hist := openHistTestDB(t)
	ctx := context.Background()
	u := seedUser(t, users, "alice")
	id, _ := hist.Create(ctx, CreateQueryHistoryParams{UserID: u.ID, CQL: "x", StatementKind: "select", Success: true})

	if err := hist.Delete(ctx, u.ID, id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := hist.Delete(ctx, u.ID, id); err == nil {
		t.Fatal("second delete should report not found")
	}
}
