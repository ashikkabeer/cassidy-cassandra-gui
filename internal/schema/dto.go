// Package schema introspects a live Cassandra/ScyllaDB cluster via the pooled
// gocql session and returns JSON-friendly DTOs for the workspace UI.
package schema

// ClusterInfo describes the cluster the workspace is talking to.
type ClusterInfo struct {
	ClusterName string `json:"cluster_name"`
	Version     string `json:"version"`
	Partitioner string `json:"partitioner"`
	LocalDC     string `json:"local_dc"`
	HostCount   int    `json:"host_count"`
	Peers       []Peer `json:"peers,omitempty"`
}

type Peer struct {
	Address string `json:"address"`
	DC      string `json:"dc,omitempty"`
	Version string `json:"version,omitempty"`
}

// Replication is a friendlier projection of system_schema.keyspaces.replication.
// `Class` is short-form (`SimpleStrategy`, `NetworkTopologyStrategy`). For
// SimpleStrategy `Factor` is the replication factor. For NetworkTopologyStrategy
// `DCFactors` maps datacenter → factor.
type Replication struct {
	Class     string            `json:"class"`
	Factor    int               `json:"factor,omitempty"`
	DCFactors map[string]int    `json:"dc_factors,omitempty"`
	Raw       map[string]string `json:"raw,omitempty"`
}

// Keyspace summarizes a keyspace row.
type Keyspace struct {
	Name          string      `json:"name"`
	DurableWrites bool        `json:"durable_writes"`
	Replication   Replication `json:"replication"`
	System        bool        `json:"system"` // true for system / system_schema / system_traces / system_auth / system_distributed
}

// TableSummary is the lightweight projection used in the schema tree (no
// columns yet — fetched on demand via Table).
type TableSummary struct {
	Name            string `json:"name"`
	Comment         string `json:"comment,omitempty"`
	DefaultTTL      int    `json:"default_ttl"`
	GCGraceSeconds  int    `json:"gc_grace"`
	CompactionClass string `json:"compaction_class,omitempty"`
}

// ColumnKind matches system_schema.columns.kind exactly.
type ColumnKind string

const (
	ColumnPartitionKey ColumnKind = "partition_key"
	ColumnClustering   ColumnKind = "clustering"
	ColumnRegular      ColumnKind = "regular"
	ColumnStatic       ColumnKind = "static"
)

// Column is a fully-resolved column row.
type Column struct {
	Name            string     `json:"name"`
	Type            string     `json:"type"`
	Kind            ColumnKind `json:"kind"`
	Position        int        `json:"position"`
	ClusteringOrder string     `json:"clustering_order,omitempty"` // "asc" / "desc" / ""
}

// Index represents a secondary index on a table.
type Index struct {
	Name    string            `json:"name"`
	Kind    string            `json:"kind"`              // CUSTOM / COMPOSITES / KEYS
	Options map[string]string `json:"options,omitempty"` // e.g. {"target": "col_name"}
}

// TableDetail is the full table metadata returned for the workspace's right pane.
type TableDetail struct {
	Keyspace         string            `json:"keyspace"`
	Name             string            `json:"name"`
	Columns          []Column          `json:"columns"`
	Indexes          []Index           `json:"indexes,omitempty"`
	Comment          string            `json:"comment,omitempty"`
	DefaultTTL       int               `json:"default_ttl"`
	GCGraceSeconds   int               `json:"gc_grace_seconds"`
	Caching          map[string]string `json:"caching,omitempty"`
	Compaction       map[string]string `json:"compaction,omitempty"`
	Compression      map[string]string `json:"compression,omitempty"`
	BloomFilterFP    float64           `json:"bloom_filter_fp_chance,omitempty"`
	SpeculativeRetry string            `json:"speculative_retry,omitempty"`
	Flags            []string          `json:"flags,omitempty"` // e.g. "compound", "counter"
}

// IsCounter reports whether the table stores counter columns.
func (t *TableDetail) IsCounter() bool {
	for _, f := range t.Flags {
		if f == "counter" {
			return true
		}
	}
	return false
}

// PartitionKey returns the partition-key columns ordered by position.
func (t *TableDetail) PartitionKey() []Column {
	return columnsOfKind(t.Columns, ColumnPartitionKey)
}

// ClusteringKey returns the clustering columns ordered by position.
func (t *TableDetail) ClusteringKey() []Column {
	return columnsOfKind(t.Columns, ColumnClustering)
}

// RegularAndStatic returns non-PK columns ordered by name (stable for DDL).
func (t *TableDetail) RegularAndStatic() []Column {
	out := []Column{}
	for _, c := range t.Columns {
		if c.Kind == ColumnRegular || c.Kind == ColumnStatic {
			out = append(out, c)
		}
	}
	return out
}

func columnsOfKind(cols []Column, kind ColumnKind) []Column {
	out := []Column{}
	for _, c := range cols {
		if c.Kind == kind {
			out = append(out, c)
		}
	}
	// Sort by position; pure-stdlib sort to avoid an import for one call.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1].Position > out[j].Position; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

// UDT describes a user-defined type.
type UDT struct {
	Name   string     `json:"name"`
	Fields []UDTField `json:"fields"`
}

type UDTField struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// IsSystemKeyspace reports whether a keyspace is one of Cassandra's built-ins
// (used to grey the keyspace icon in the UI and to filter "system" by default).
func IsSystemKeyspace(name string) bool {
	switch name {
	case "system", "system_schema", "system_auth", "system_distributed", "system_traces", "system_virtual_schema", "system_views":
		return true
	}
	return false
}
