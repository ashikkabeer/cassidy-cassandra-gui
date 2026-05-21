package query

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/apache/cassandra-gocql-driver/v2"
	"github.com/ashikkabeer/cassandra-gui/internal/cluster"
	"github.com/ashikkabeer/cassandra-gui/internal/connections"
	"github.com/ashikkabeer/cassandra-gui/internal/metastore"
	"github.com/ashikkabeer/cassandra-gui/internal/schema"
)

// Errors flow up to the HTTP layer's mapper for {error: {code, message}}.
var (
	ErrReadOnlyConnection = errors.New("connection is marked read-only")
	ErrInvalidPageState   = errors.New("invalid page_state — re-run the query")
)

// Service orchestrates CQL execution and history recording.
type Service struct {
	conns    *connections.Service
	mgr      *cluster.Manager
	schema   *schema.Service
	history  *metastore.QueryHistory
	maxRows  int
	maxPage  int
	historyW chan historyRecord // async channel so history writes never block /query
}

// NewService wires the query service. `maxRows` caps a single response (the
// export endpoint streams beyond this). `historyBuffer` sizes the async
// history channel; 64 is plenty for human-driven workloads.
func NewService(conns *connections.Service, mgr *cluster.Manager, sch *schema.Service, history *metastore.QueryHistory, maxRows int) *Service {
	if maxRows <= 0 {
		maxRows = 5000
	}
	s := &Service{
		conns:    conns,
		mgr:      mgr,
		schema:   sch,
		history:  history,
		maxRows:  maxRows,
		maxPage:  1000,
		historyW: make(chan historyRecord, 64),
	}
	go s.historyWriter()
	return s
}

// historyRecord is the message the writer goroutine consumes.
type historyRecord struct {
	p metastore.CreateQueryHistoryParams
}

func (s *Service) historyWriter() {
	for r := range s.historyW {
		// Best-effort: a slow disk should never block the user's response.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if _, err := s.history.Create(ctx, r.p); err != nil {
			slog.Warn("query history write failed", "err", err)
		}
		cancel()
	}
}

// Run executes a single CQL statement and returns one page of results.
func (s *Service) Run(ctx context.Context, userID, ownerID, connID string, req RunRequest) (*QueryResult, error) {
	// 1. Authorize + load the saved connection.
	conn, err := s.conns.Get(ctx, ownerID, connID)
	if err != nil {
		return nil, err
	}

	// 2. Classify the statement. Multi-statement / unsupported / empty are
	//    rejected here without any cluster IO.
	kind, classifyErr := Classify(req.CQL)
	if classifyErr != nil {
		s.recordHistory(userID, connID, req, string(kind), false, errorCodeOf(classifyErr), classifyErr.Error(), 0, 0)
		return nil, classifyErr
	}

	// 3. Read-only enforcement.
	if conn.ReadOnly && !IsReadOnly(kind) {
		s.recordHistory(userID, connID, req, string(kind), false, "read_only_connection", ErrReadOnlyConnection.Error(), 0, 0)
		return nil, ErrReadOnlyConnection
	}

	// 4. Get the pooled session.
	sess, err := s.mgr.GetSession(ctx, conn)
	if err != nil {
		return nil, err
	}

	// 5. Resolve paging + consistency.
	pageSize := req.PageSize
	if pageSize <= 0 || pageSize > s.maxPage {
		pageSize = 100
	}
	var prevPageState []byte
	if req.PageState != "" {
		prevPageState, err = base64.RawURLEncoding.DecodeString(req.PageState)
		if err != nil {
			return nil, ErrInvalidPageState
		}
	}

	// 6. Per-request timeout.
	timeout := time.Duration(conn.RequestTimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	qCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 7. Build the query.
	q := sess.Query(req.CQL).WithContext(qCtx).PageSize(pageSize)
	if prevPageState != nil {
		q = q.PageState(prevPageState)
	}
	q = q.Idempotent(IsReadOnly(kind))
	if req.Consistency != "" {
		q = q.Consistency(gocql.ParseConsistency(strings.ToUpper(req.Consistency)))
	}

	// 8. Iterate one logical page. gocql.Iter auto-pages internally so we
	//    stop the loop after pageSize rows; PageState() then captures the
	//    cursor for the next request. maxRows is the belt-and-braces memory
	//    cap (in practice it's always > pageSize).
	start := time.Now()
	iter := q.Iter()
	defer iter.Close()

	rowCap := pageSize
	if rowCap > s.maxRows {
		rowCap = s.maxRows
	}
	rows := make([][]any, 0, rowCap)
	columns := iter.Columns()
	for len(rows) < rowCap {
		raw := map[string]any{}
		if !iter.MapScan(raw) {
			break
		}
		rows = append(rows, jsonRow(columns, raw))
	}

	pageState := iter.PageState()
	warnings := iter.Warnings()

	closeErr := iter.Close()
	durationMS := time.Since(start).Milliseconds()
	if closeErr != nil {
		code, msg := mapGocqlError(closeErr, qCtx.Err())
		s.recordHistory(userID, connID, req, string(kind), false, code, msg, len(rows), int(durationMS))
		return nil, &runError{Code: code, Message: msg, Err: closeErr}
	}

	res := &QueryResult{
		Columns:       toColumnInfos(columns),
		Rows:          rows,
		Applied:       true,
		RowCount:      len(rows),
		DurationMS:    durationMS,
		Warnings:      warnings,
		StatementKind: string(kind),
	}
	if len(pageState) > 0 {
		res.NextPageState = base64.RawURLEncoding.EncodeToString(pageState)
	}

	// 9. DDL invalidates the schema cache so the workspace tree refreshes.
	if IsDDL(kind) && s.schema != nil {
		s.schema.InvalidateConn(connID)
	}

	// 10. Record history (fire-and-forget).
	s.recordHistory(userID, connID, req, string(kind), true, "", "", len(rows), int(durationMS))

	// 11. Touch last_used_at.
	s.conns.TouchLastUsed(connID)

	return res, nil
}

// recordHistory enqueues a history-write to the async writer.
func (s *Service) recordHistory(userID, connID string, req RunRequest, kind string, success bool, errCode, errMsg string, rowCount, durMS int) {
	if s.history == nil {
		return
	}
	select {
	case s.historyW <- historyRecord{p: metastore.CreateQueryHistoryParams{
		UserID:        userID,
		ConnectionID:  connID,
		Keyspace:      req.Keyspace,
		CQL:           req.CQL,
		StatementKind: kind,
		Success:       success,
		ErrorCode:     errCode,
		ErrorMessage:  errMsg,
		RowCount:      rowCount,
		DurationMS:    durMS,
	}}:
	default:
		// Channel full — drop the entry rather than block the user. With 64
		// buffer this only happens under sustained high load; acceptable.
		slog.Warn("query history channel full; dropping entry", "user_id", userID)
	}
}

// runError lets the HTTP layer attach a stable error code per failure mode.
type runError struct {
	Code    string
	Message string
	Err     error
}

func (e *runError) Error() string { return e.Message }
func (e *runError) Unwrap() error { return e.Err }

// CodeOf reports the stable error code for an error from Run, used by the HTTP
// layer to map to the right JSON envelope + HTTP status.
func CodeOf(err error) string {
	if err == nil {
		return ""
	}
	var re *runError
	if errors.As(err, &re) {
		return re.Code
	}
	switch {
	case errors.Is(err, ErrReadOnlyConnection):
		return "read_only_connection"
	case errors.Is(err, ErrMultiStatement):
		return "multi_statement"
	case errors.Is(err, ErrUnsupported):
		return "unsupported_statement"
	case errors.Is(err, ErrInvalidPageState):
		return "invalid_page_state"
	case errors.Is(err, ErrEmpty):
		return "empty_statement"
	case errors.Is(err, metastore.ErrConnectionNotFound):
		return "connection_not_found"
	}
	return "cluster_unreachable"
}

// errorCodeOf is the recordHistory side of CodeOf — same mapping but only
// classifies the classifier errors. The HTTP layer uses CodeOf above.
func errorCodeOf(err error) string {
	switch {
	case errors.Is(err, ErrMultiStatement):
		return "multi_statement"
	case errors.Is(err, ErrUnsupported):
		return "unsupported_statement"
	case errors.Is(err, ErrEmpty):
		return "empty_statement"
	}
	return "cql_error"
}

// mapGocqlError turns a gocql error + context error into a stable code + msg
// the frontend can branch on.
func mapGocqlError(closeErr, ctxErr error) (code, msg string) {
	if ctxErr != nil {
		// The query was cancelled because our timeout fired.
		switch {
		case errors.Is(ctxErr, context.DeadlineExceeded):
			return "query_timeout", "query exceeded the request timeout"
		case errors.Is(ctxErr, context.Canceled):
			return "query_cancelled", "query was cancelled"
		}
	}
	if closeErr == nil {
		return "", ""
	}
	// gocql wraps server-side errors with detailed messages already.
	return "cql_error", closeErr.Error()
}

// ListHistory returns the caller's query history, optionally filtered.
type HistoryFilters struct {
	ConnectionID string
	Kind         string
	Success      *bool
	Limit        int
	Before       string
}

func (s *Service) ListHistory(ctx context.Context, userID string, f HistoryFilters) ([]HistoryEntry, error) {
	rows, err := s.history.ListByUser(ctx, userID, metastore.ListByUserFilter{
		ConnectionID: f.ConnectionID,
		Kind:         f.Kind,
		Success:      f.Success,
		Limit:        f.Limit,
		Before:       f.Before,
	})
	if err != nil {
		return nil, err
	}
	out := make([]HistoryEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, toHistoryDTO(r))
	}
	return out, nil
}

func (s *Service) DeleteHistory(ctx context.Context, userID, id string) error {
	return s.history.Delete(ctx, userID, id)
}

func toHistoryDTO(r metastore.QueryHistoryEntry) HistoryEntry {
	e := HistoryEntry{
		ID:            r.ID,
		CQL:           r.CQL,
		StatementKind: r.StatementKind,
		Success:       r.Success,
		RowCount:      r.RowCount,
		DurationMS:    r.DurationMS,
		ExecutedAt:    r.ExecutedAt.UTC().Format(time.RFC3339),
	}
	if r.ConnectionID.Valid {
		e.ConnectionID = r.ConnectionID.String
	}
	if r.Keyspace.Valid {
		e.Keyspace = r.Keyspace.String
	}
	if r.ErrorCode.Valid {
		e.ErrorCode = r.ErrorCode.String
	}
	if r.ErrorMessage.Valid {
		e.ErrorMessage = r.ErrorMessage.String
	}
	return e
}

// nullable is a guard for the schema service's connection lookup which
// returns sql.NullString for unset columns. Kept tiny / unexported.
var _ = sql.NullString{}

// Trim helper used by classifier-adjacent code if needed.
func trimSpaces(s string) string { return strings.TrimSpace(s) }

// IsZeroAny reports whether the empty interface is its zero value (used by
// future test helpers; kept here so the marshal_test file doesn't need a
// separate utility). Not exported because it has no production use.
func isZeroAny(v any) bool { return v == nil || fmt.Sprintf("%v", v) == "" }
