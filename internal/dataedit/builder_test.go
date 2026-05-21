package dataedit

import (
	"errors"
	"strings"
	"testing"

	"github.com/ashikkabeer/cassandra-gui/internal/schema"
)

// sampleTable: composite PK (device_id, bucket), clustering ts, regular cols
// incl. a counter and a collection to exercise the locked-column paths.
func sampleTable() *schema.TableDetail {
	return &schema.TableDetail{
		Keyspace: "telemetry",
		Name:     "readings",
		Columns: []schema.Column{
			{Name: "device_id", Type: "uuid", Kind: schema.ColumnPartitionKey, Position: 0},
			{Name: "bucket", Type: "date", Kind: schema.ColumnPartitionKey, Position: 1},
			{Name: "ts", Type: "timestamp", Kind: schema.ColumnClustering, Position: 0, ClusteringOrder: "desc"},
			{Name: "temperature_c", Type: "float", Kind: schema.ColumnRegular},
			{Name: "firmware", Type: "text", Kind: schema.ColumnRegular},
			{Name: "hits", Type: "counter", Kind: schema.ColumnRegular},
			{Name: "tags", Type: "set<text>", Kind: schema.ColumnRegular},
		},
	}
}

const (
	devID = "7c4a3b2e-9d18-4a8a-b3e1-39b22c14a55b"
)

func fullPK() map[string]any {
	return map[string]any{
		"device_id": devID,
		"bucket":    "2026-05-20",
		"ts":        "2026-05-20T14:58:02Z",
	}
}

func TestBuildUpdate_HappyPath(t *testing.T) {
	st, err := BuildUpdate(sampleTable(), Op{
		Kind: "update",
		PK:   fullPK(),
		Set:  map[string]any{"firmware": "1.4.0-rc2", "temperature_c": 21.5},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if !strings.HasPrefix(st.cql, "UPDATE telemetry.readings SET ") {
		t.Fatalf("bad cql: %s", st.cql)
	}
	// WHERE must AND all three PK/CK columns.
	for _, frag := range []string{"device_id = ?", "bucket = ?", "ts = ?", " AND "} {
		if !strings.Contains(st.cql, frag) {
			t.Fatalf("WHERE missing %q in %s", frag, st.cql)
		}
	}
	// args = 2 SET + 3 WHERE = 5.
	if len(st.args) != 5 {
		t.Fatalf("expected 5 args, got %d", len(st.args))
	}
	// display inlines the firmware literal, escaped/quoted.
	if !strings.Contains(st.display, "firmware = '1.4.0-rc2'") {
		t.Fatalf("display missing inlined firmware: %s", st.display)
	}
}

func TestBuildUpdate_MissingClusteringKeyRejected(t *testing.T) {
	pk := fullPK()
	delete(pk, "ts") // drop the clustering key
	_, err := BuildUpdate(sampleTable(), Op{Kind: "update", PK: pk, Set: map[string]any{"firmware": "x-aaaaaaaaaa"}})
	if !errors.Is(err, ErrIncompletePK) {
		t.Fatalf("expected ErrIncompletePK, got %v", err)
	}
}

func TestBuildUpdate_MissingPartitionKeyRejected(t *testing.T) {
	pk := fullPK()
	delete(pk, "bucket")
	_, err := BuildUpdate(sampleTable(), Op{Kind: "update", PK: pk, Set: map[string]any{"firmware": "x-aaaaaaaaaa"}})
	if !errors.Is(err, ErrIncompletePK) {
		t.Fatalf("expected ErrIncompletePK, got %v", err)
	}
}

func TestBuildUpdate_EmptySetRejected(t *testing.T) {
	_, err := BuildUpdate(sampleTable(), Op{Kind: "update", PK: fullPK(), Set: map[string]any{}})
	if !errors.Is(err, ErrNoChanges) {
		t.Fatalf("expected ErrNoChanges, got %v", err)
	}
}

func TestBuildUpdate_CounterRejected(t *testing.T) {
	_, err := BuildUpdate(sampleTable(), Op{Kind: "update", PK: fullPK(), Set: map[string]any{"hits": "5"}})
	if !errors.Is(err, ErrUnsupportedColumn) {
		t.Fatalf("expected ErrUnsupportedColumn for counter, got %v", err)
	}
}

func TestBuildUpdate_CollectionRejected(t *testing.T) {
	_, err := BuildUpdate(sampleTable(), Op{Kind: "update", PK: fullPK(), Set: map[string]any{"tags": "{a,b}"}})
	if !errors.Is(err, ErrUnsupportedColumn) {
		t.Fatalf("expected ErrUnsupportedColumn for set<text>, got %v", err)
	}
}

func TestBuildDelete_RequiresFullPK(t *testing.T) {
	pk := fullPK()
	delete(pk, "ts")
	_, err := BuildDelete(sampleTable(), Op{Kind: "delete", PK: pk})
	if !errors.Is(err, ErrIncompletePK) {
		t.Fatalf("expected ErrIncompletePK, got %v", err)
	}

	st, err := BuildDelete(sampleTable(), Op{Kind: "delete", PK: fullPK()})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if !strings.HasPrefix(st.cql, "DELETE FROM telemetry.readings WHERE ") {
		t.Fatalf("bad delete cql: %s", st.cql)
	}
	if len(st.args) != 3 {
		t.Fatalf("expected 3 WHERE args, got %d", len(st.args))
	}
}

func TestBuildInsert_RequiresFullPK(t *testing.T) {
	// Missing bucket → reject.
	_, err := BuildInsert(sampleTable(), Op{
		Kind: "insert",
		PK:   map[string]any{"device_id": devID, "ts": "2026-05-20T14:58:02Z"},
		Set:  map[string]any{"firmware": "1.0.0-aaaaa"},
	})
	if !errors.Is(err, ErrIncompletePK) {
		t.Fatalf("expected ErrIncompletePK, got %v", err)
	}

	st, err := BuildInsert(sampleTable(), Op{
		Kind: "insert",
		PK:   fullPK(),
		Set:  map[string]any{"firmware": "1.0.0-aaaaa", "temperature_c": 20.0},
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if !strings.HasPrefix(st.cql, "INSERT INTO telemetry.readings (") {
		t.Fatalf("bad insert cql: %s", st.cql)
	}
	// 3 PK + 2 set = 5 columns / placeholders / args.
	if strings.Count(st.cql, "?") != 5 || len(st.args) != 5 {
		t.Fatalf("expected 5 placeholders+args, got %d ? / %d args", strings.Count(st.cql, "?"), len(st.args))
	}
}

func TestBuildInsert_StringEscaping(t *testing.T) {
	st, err := BuildInsert(sampleTable(), Op{
		Kind: "insert",
		PK:   fullPK(),
		Set:  map[string]any{"firmware": "it's v2"},
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if !strings.Contains(st.display, "'it''s v2'") {
		t.Fatalf("display did not escape single quote: %s", st.display)
	}
}
