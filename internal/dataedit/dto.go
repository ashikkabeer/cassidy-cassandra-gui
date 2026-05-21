// Package dataedit implements the table-data browse + edit feature: paged row
// fetch and a structured-changeset commit path that builds parameterized
// INSERT/UPDATE/DELETE statements (full-PK-validated) and runs them in a single
// atomic LOGGED BATCH.
package dataedit

// ColumnMeta describes a column for the editable grid: its CQL type, key kind,
// and whether the inline grid may edit it (false for PK/clustering/counter/
// collection columns).
type ColumnMeta struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Kind     string `json:"kind"` // partition_key | clustering | regular | static
	Editable bool   `json:"editable"`
}

// RowsPage is one page of rows for the data tab.
type RowsPage struct {
	Columns       []ColumnMeta `json:"columns"`
	Rows          [][]any      `json:"rows"`
	NextPageState string       `json:"next_page_state,omitempty"`
}

// Op is one staged change. PK carries the full primary key (partition +
// clustering). Set carries the changed non-PK columns (update) or is empty
// (delete). For insert, the full row is PK ∪ Set.
type Op struct {
	Kind string         `json:"kind"` // "insert" | "update" | "delete"
	PK   map[string]any `json:"pk"`
	Set  map[string]any `json:"set,omitempty"`
}

// ChangeSet is the body of /rows/preview and /rows/commit.
type ChangeSet struct {
	Ops []Op `json:"ops"`
}

// PreviewResponse renders the BATCH that a commit would run.
type PreviewResponse struct {
	CQL            string `json:"cql"`
	DeleteCount    int    `json:"delete_count"`
	StatementCount int    `json:"statement_count"`
}

// CommitResponse is returned after a successful commit.
type CommitResponse struct {
	Applied        bool `json:"applied"`
	StatementCount int  `json:"statement_count"`
}
