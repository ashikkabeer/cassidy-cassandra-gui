package schema

import "testing"

func TestBuildCreateTable_CompositePKAndClusteringDesc(t *testing.T) {
	tb := &TableDetail{
		Keyspace: "telemetry",
		Name:     "sensor_readings",
		Columns: []Column{
			{Name: "device_id", Type: "uuid", Kind: ColumnPartitionKey, Position: 0},
			{Name: "bucket", Type: "date", Kind: ColumnPartitionKey, Position: 1},
			{Name: "ts", Type: "timestamp", Kind: ColumnClustering, Position: 0, ClusteringOrder: "desc"},
			{Name: "temperature_c", Type: "float", Kind: ColumnRegular, Position: -1},
			{Name: "humidity_pct", Type: "float", Kind: ColumnRegular, Position: -1},
			{Name: "tags", Type: "set<text>", Kind: ColumnRegular, Position: -1},
		},
		GCGraceSeconds: 86400,
		DefaultTTL:     2592000,
		Compaction: map[string]string{
			"class":                  "TimeWindowCompactionStrategy",
			"compaction_window_size": "1",
			"compaction_window_unit": "DAYS",
		},
	}
	got := BuildCreateTable(tb)

	wantContains := []string{
		"CREATE TABLE telemetry.sensor_readings (",
		"device_id uuid,",
		"bucket date,",
		"ts timestamp,",
		"humidity_pct float,",
		"temperature_c float,",
		"tags set<text>,",
		"PRIMARY KEY ((device_id, bucket), ts)",
		") WITH ",
		"CLUSTERING ORDER BY (ts DESC)",
		"compaction = {'class': 'TimeWindowCompactionStrategy'",
		"gc_grace_seconds = 86400",
		"default_time_to_live = 2592000",
		";",
	}
	for _, w := range wantContains {
		if !contains(got, w) {
			t.Fatalf("CREATE TABLE missing %q\n--- got ---\n%s", w, got)
		}
	}
}

func TestBuildCreateTable_SinglePartitionKeyNoClustering(t *testing.T) {
	tb := &TableDetail{
		Keyspace: "telemetry",
		Name:     "devices",
		Columns: []Column{
			{Name: "device_id", Type: "uuid", Kind: ColumnPartitionKey, Position: 0},
			{Name: "model", Type: "text", Kind: ColumnRegular},
			{Name: "serial", Type: "text", Kind: ColumnRegular},
		},
	}
	got := BuildCreateTable(tb)

	if contains(got, "((device_id))") {
		t.Fatalf("single-PK table must NOT wrap the column in extra parens; got:\n%s", got)
	}
	if !contains(got, "PRIMARY KEY (device_id)") {
		t.Fatalf("expected PRIMARY KEY (device_id); got:\n%s", got)
	}
	if contains(got, "CLUSTERING ORDER") {
		t.Fatalf("no-clustering table should not emit CLUSTERING ORDER clause; got:\n%s", got)
	}
}

func TestBuildCreateTable_StaticColumn(t *testing.T) {
	tb := &TableDetail{
		Keyspace: "rooms",
		Name:     "messages",
		Columns: []Column{
			{Name: "room_id", Type: "uuid", Kind: ColumnPartitionKey, Position: 0},
			{Name: "sent_at", Type: "timestamp", Kind: ColumnClustering, Position: 0},
			{Name: "room_name", Type: "text", Kind: ColumnStatic},
			{Name: "body", Type: "text", Kind: ColumnRegular},
		},
	}
	got := BuildCreateTable(tb)

	if !contains(got, "room_name text static,") {
		t.Fatalf("expected `room_name text static,` in DDL; got:\n%s", got)
	}
}

func TestBuildCreateTable_QuotesMixedCaseIdent(t *testing.T) {
	tb := &TableDetail{
		Keyspace: "MyKs",
		Name:     "MixedCase",
		Columns: []Column{
			{Name: "Id", Type: "uuid", Kind: ColumnPartitionKey, Position: 0},
		},
	}
	got := BuildCreateTable(tb)
	for _, w := range []string{`"MyKs"`, `"MixedCase"`, `"Id"`} {
		if !contains(got, w) {
			t.Fatalf("expected %s to be quoted; got:\n%s", w, got)
		}
	}
}

func TestQuoteIdent(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"device_id", "device_id"},
		{"_underscore", "_underscore"},
		{"MixedCase", `"MixedCase"`},
		{"with space", `"with space"`},
		{"has\"quote", `"has""quote"`},
	}
	for _, c := range cases {
		if got := quoteIdent(c.in); got != c.want {
			t.Errorf("quoteIdent(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
