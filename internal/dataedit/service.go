package dataedit

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/apache/cassandra-gocql-driver/v2"
	"github.com/ashikkabeer/cassandra-gui/internal/cluster"
	"github.com/ashikkabeer/cassandra-gui/internal/connections"
	"github.com/ashikkabeer/cassandra-gui/internal/metastore"
	"github.com/ashikkabeer/cassandra-gui/internal/query"
	"github.com/ashikkabeer/cassandra-gui/internal/schema"
)

// ErrReadOnly mirrors query's read-only sentinel so the HTTP layer can map it
// to the same `read_only_connection` code the query path uses.
var ErrReadOnly = errors.New("connection is marked read-only")

// ErrInvalidPageState is returned when the supplied page_state can't be decoded.
var ErrInvalidPageState = errors.New("invalid page_state — reload the table")

// Service handles the data-tab read path and the changeset commit path.
type Service struct {
	conns   *connections.Service
	mgr     *cluster.Manager
	schema  *schema.Service
	history *metastore.QueryHistory
}

func NewService(conns *connections.Service, mgr *cluster.Manager, sch *schema.Service, history *metastore.QueryHistory) *Service {
	return &Service{conns: conns, mgr: mgr, schema: sch, history: history}
}

// Rows returns one page of `SELECT * FROM ks.tbl`, enriched with per-column
// editability derived from the table's schema.
func (s *Service) Rows(ctx context.Context, ownerID, connID, ks, tbl string, pageSize int, pageStateB64 string) (*RowsPage, error) {
	conn, err := s.conns.Get(ctx, ownerID, connID)
	if err != nil {
		return nil, err
	}
	td, err := s.schema.Table(ctx, ownerID, connID, ks, tbl)
	if err != nil {
		return nil, err
	}
	sess, err := s.mgr.GetSession(ctx, conn)
	if err != nil {
		return nil, err
	}

	if pageSize <= 0 || pageSize > 1000 {
		pageSize = 100
	}
	var prev []byte
	if pageStateB64 != "" {
		prev, err = base64.RawURLEncoding.DecodeString(pageStateB64)
		if err != nil {
			return nil, ErrInvalidPageState
		}
	}

	timeout := time.Duration(conn.RequestTimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	qCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cql := fmt.Sprintf("SELECT * FROM %s.%s", quoteIdent(ks), quoteIdent(tbl))
	q := sess.Query(cql).WithContext(qCtx).PageSize(pageSize).Idempotent(true)
	if prev != nil {
		q = q.PageState(prev)
	}
	iter := q.Iter()
	defer iter.Close()

	cols := iter.Columns()
	rows := make([][]any, 0, pageSize)
	for len(rows) < pageSize {
		raw := map[string]any{}
		if !iter.MapScan(raw) {
			break
		}
		rows = append(rows, query.JSONRow(cols, raw))
	}
	ps := iter.PageState()
	if err := iter.Close(); err != nil {
		return nil, err
	}

	page := &RowsPage{
		Columns: columnMeta(td, cols),
		Rows:    rows,
	}
	if len(ps) > 0 {
		page.NextPageState = base64.RawURLEncoding.EncodeToString(ps)
	}
	s.conns.TouchLastUsed(connID)
	return page, nil
}

// columnMeta maps the result columns (in result order) to ColumnMeta enriched
// with kind + editability from the table schema.
func columnMeta(td *schema.TableDetail, cols []gocql.ColumnInfo) []ColumnMeta {
	byName := map[string]schema.Column{}
	for _, c := range td.Columns {
		byName[c.Name] = c
	}
	out := make([]ColumnMeta, 0, len(cols))
	for _, c := range cols {
		sc, ok := byName[c.Name]
		kind := "regular"
		editable := false
		typ := ""
		if ok {
			kind = string(sc.Kind)
			typ = sc.Type
			editable = isEditable(sc)
		}
		out = append(out, ColumnMeta{Name: c.Name, Type: typ, Kind: kind, Editable: editable})
	}
	return out
}

// Preview builds the BATCH that a commit would run, without touching the cluster.
func (s *Service) Preview(ctx context.Context, ownerID, connID, ks, tbl string, cs ChangeSet) (*PreviewResponse, error) {
	if _, err := s.conns.Get(ctx, ownerID, connID); err != nil {
		return nil, err
	}
	td, err := s.schema.Table(ctx, ownerID, connID, ks, tbl)
	if err != nil {
		return nil, err
	}
	stmts, deletes, err := s.buildAll(td, cs)
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	b.WriteString("BEGIN BATCH\n")
	for _, st := range stmts {
		b.WriteString("  ")
		b.WriteString(st.display)
		b.WriteString(";\n")
	}
	b.WriteString("APPLY BATCH;")
	return &PreviewResponse{CQL: b.String(), DeleteCount: deletes, StatementCount: len(stmts)}, nil
}

// Commit builds + executes the changeset as a single logged batch.
func (s *Service) Commit(ctx context.Context, userID, ownerID, connID, ks, tbl string, cs ChangeSet) (*CommitResponse, error) {
	conn, err := s.conns.Get(ctx, ownerID, connID)
	if err != nil {
		return nil, err
	}
	if conn.ReadOnly {
		return nil, ErrReadOnly
	}
	td, err := s.schema.Table(ctx, ownerID, connID, ks, tbl)
	if err != nil {
		return nil, err
	}
	stmts, _, err := s.buildAll(td, cs)
	if err != nil {
		return nil, err
	}
	if len(stmts) == 0 {
		return &CommitResponse{Applied: true, StatementCount: 0}, nil
	}
	sess, err := s.mgr.GetSession(ctx, conn)
	if err != nil {
		return nil, err
	}

	timeout := time.Duration(conn.RequestTimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	bCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	batch := sess.Batch(gocql.LoggedBatch)
	for _, st := range stmts {
		batch.Query(st.cql, st.args...)
	}
	start := time.Now()
	execErr := batch.ExecContext(bCtx)
	durMS := int(time.Since(start).Milliseconds())

	// Record to history as a batch (best-effort, synchronous — commits are rare).
	if s.history != nil {
		previewCQL := ""
		if p, perr := s.Preview(ctx, ownerID, connID, ks, tbl, cs); perr == nil {
			previewCQL = p.CQL
		}
		_, _ = s.history.Create(context.Background(), metastore.CreateQueryHistoryParams{
			UserID:        userID,
			ConnectionID:  connID,
			Keyspace:      ks,
			CQL:           previewCQL,
			StatementKind: "batch",
			Success:       execErr == nil,
			ErrorMessage:  errString(execErr),
			RowCount:      0,
			DurationMS:    durMS,
		})
	}

	if execErr != nil {
		return nil, fmt.Errorf("batch: %w", execErr)
	}
	s.conns.TouchLastUsed(connID)
	return &CommitResponse{Applied: true, StatementCount: len(stmts)}, nil
}

// buildAll turns a changeset into ordered statements + a delete count.
func (s *Service) buildAll(td *schema.TableDetail, cs ChangeSet) ([]statement, int, error) {
	stmts := make([]statement, 0, len(cs.Ops))
	deletes := 0
	for i, op := range cs.Ops {
		var (
			st  statement
			err error
		)
		switch op.Kind {
		case "insert":
			st, err = BuildInsert(td, op)
		case "update":
			st, err = BuildUpdate(td, op)
		case "delete":
			st, err = BuildDelete(td, op)
			deletes++
		default:
			return nil, 0, fmt.Errorf("op %d: unknown kind %q", i, op.Kind)
		}
		if err != nil {
			return nil, 0, fmt.Errorf("op %d (%s): %w", i, op.Kind, err)
		}
		stmts = append(stmts, st)
	}
	return stmts, deletes, nil
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
