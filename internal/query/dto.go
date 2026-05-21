package query

// RunRequest is the body of POST /connections/{id}/query.
type RunRequest struct {
	CQL         string `json:"cql"`
	Keyspace    string `json:"keyspace,omitempty"`    // hint only; ignored server-side in M4 (see plan)
	PageSize    int    `json:"page_size,omitempty"`   // default 100, max 1000
	PageState   string `json:"page_state,omitempty"`  // base64-encoded; opaque; supplied by client from the previous response
	Consistency string `json:"consistency,omitempty"` // optional override, else falls back to connection default
}

// ColumnInfo describes one column in the result set.
type ColumnInfo struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// QueryResult is the response shape for /query. `Applied` is true for
// successful DDL/DML where no rows are returned. `Rows` carries JSON-friendly
// values via the marshal package.
type QueryResult struct {
	Columns       []ColumnInfo `json:"columns"`
	Rows          [][]any      `json:"rows"`
	NextPageState string       `json:"next_page_state,omitempty"`
	Applied       bool         `json:"applied"`
	RowCount      int          `json:"row_count"`
	DurationMS    int64        `json:"duration_ms"`
	Warnings      []string     `json:"warnings,omitempty"`
	StatementKind string       `json:"statement_kind"`
}

// CompletionKind labels suggestions for the editor's autocomplete dropdown.
type CompletionKind string

const (
	CompletionKeyspace CompletionKind = "keyspace"
	CompletionTable    CompletionKind = "table"
	CompletionColumn   CompletionKind = "column"
	CompletionKeyword  CompletionKind = "keyword"
	CompletionType     CompletionKind = "type"
	CompletionFunction CompletionKind = "function"
)

// CompletionSuggestion is one row in the completion popover.
type CompletionSuggestion struct {
	Label  string         `json:"label"`
	Detail string         `json:"detail,omitempty"`
	Kind   CompletionKind `json:"kind"`
}

// HistoryEntry is the wire shape for /query-history.
type HistoryEntry struct {
	ID            string `json:"id"`
	ConnectionID  string `json:"connection_id,omitempty"`
	Keyspace      string `json:"keyspace,omitempty"`
	CQL           string `json:"cql"`
	StatementKind string `json:"statement_kind"`
	Success       bool   `json:"success"`
	ErrorCode     string `json:"error_code,omitempty"`
	ErrorMessage  string `json:"error_message,omitempty"`
	RowCount      int    `json:"row_count"`
	DurationMS    int    `json:"duration_ms"`
	ExecutedAt    string `json:"executed_at"`
}
