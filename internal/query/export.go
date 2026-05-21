package query

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/apache/cassandra-gocql-driver/v2"
)

// ExportFormat is the wire format the caller asked for.
type ExportFormat string

const (
	ExportCSV  ExportFormat = "csv"
	ExportJSON ExportFormat = "json" // NDJSON, one object per line
)

// Export executes the supplied CQL and streams the rows to the caller-supplied
// writer one row at a time. Unlike Run, Export does NOT cap rows — it iterates
// the gocql Iter to exhaustion (paging happens internally), flushing after each
// row so HTTP chunked encoding keeps memory bounded. Read-only enforcement and
// the statement classifier are applied identically.
func (s *Service) Export(ctx context.Context, userID, ownerID, connID string, req RunRequest, format ExportFormat, w http.ResponseWriter) error {
	// Authorize + load.
	conn, err := s.conns.Get(ctx, ownerID, connID)
	if err != nil {
		return err
	}
	kind, classifyErr := Classify(req.CQL)
	if classifyErr != nil {
		return classifyErr
	}
	if conn.ReadOnly && !IsReadOnly(kind) {
		return ErrReadOnlyConnection
	}
	// Export only makes sense for SELECTs in M4 (other kinds return no rows
	// anyway). Documented behaviour: non-SELECT executes once and the response
	// is an empty stream + the column header line if CSV.
	sess, err := s.mgr.GetSession(ctx, conn)
	if err != nil {
		return err
	}

	timeout := time.Duration(conn.RequestTimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 30 * time.Second // exports are typically heavier than /query
	}
	qCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	q := sess.Query(req.CQL).WithContext(qCtx).PageSize(500).Idempotent(true)
	if req.Consistency != "" {
		q = q.Consistency(gocql.ParseConsistency(strings.ToUpper(req.Consistency)))
	}

	iter := q.Iter()
	defer iter.Close()
	cols := iter.Columns()

	flusher, _ := w.(http.Flusher)

	switch format {
	case ExportCSV:
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=cassidy-export.csv")
		cw := csv.NewWriter(w)
		header := make([]string, len(cols))
		for i, c := range cols {
			header[i] = c.Name
		}
		if err := cw.Write(header); err != nil {
			return err
		}
		raw := map[string]any{}
		for iter.MapScan(raw) {
			row := make([]string, len(cols))
			for i, c := range cols {
				row[i] = csvify(jsonValue(raw[c.Name]))
			}
			if err := cw.Write(row); err != nil {
				return err
			}
			cw.Flush()
			if flusher != nil {
				flusher.Flush()
			}
			raw = map[string]any{}
		}
		cw.Flush()
	case ExportJSON:
		w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=cassidy-export.jsonl")
		enc := json.NewEncoder(w)
		raw := map[string]any{}
		for iter.MapScan(raw) {
			obj := make(map[string]any, len(cols))
			for _, c := range cols {
				obj[c.Name] = jsonValue(raw[c.Name])
			}
			if err := enc.Encode(obj); err != nil {
				return err
			}
			if flusher != nil {
				flusher.Flush()
			}
			raw = map[string]any{}
		}
	default:
		return fmt.Errorf("unsupported export format %q", format)
	}

	if err := iter.Close(); err != nil {
		// Headers and some body are already written; we can't change the
		// status code now. Log it; the client sees a truncated stream.
		return err
	}
	// Touch last_used_at + record history (best-effort).
	s.conns.TouchLastUsed(connID)
	s.recordHistory(userID, connID, req, string(kind), true, "", "", -1, 0)

	_ = io.Discard // silence the unused import if json/csv get removed; keep for safety
	return nil
}

// csvify renders a JSON-friendly value as a CSV-safe string. nil becomes "".
func csvify(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	}
	b, _ := json.Marshal(v)
	return string(b)
}
