package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/ashikkabeer/cassandra-gui/internal/auth"
	"github.com/ashikkabeer/cassandra-gui/internal/metastore"
	"github.com/ashikkabeer/cassandra-gui/internal/query"
	"github.com/go-chi/chi/v5"
)

// httpStatusForQueryCode maps a stable query error code to an HTTP status.
func httpStatusForQueryCode(code string) int {
	switch code {
	case "read_only_connection":
		return http.StatusForbidden
	case "multi_statement", "unsupported_statement", "empty_statement", "invalid_page_state", "cql_error":
		return http.StatusBadRequest
	case "connection_not_found":
		return http.StatusNotFound
	case "query_timeout":
		return http.StatusGatewayTimeout
	case "cluster_unreachable":
		return http.StatusBadGateway
	}
	return http.StatusInternalServerError
}

// writeQueryError emits the uniform JSON envelope with the right HTTP status.
func writeQueryError(w http.ResponseWriter, err error) {
	code := query.CodeOf(err)
	writeError(w, httpStatusForQueryCode(code), code, err.Error())
}

// ─── POST /connections/{id}/query ────────────────────────────────────────

func (s *Server) handleRunQuery(w http.ResponseWriter, r *http.Request) {
	caller, _ := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")

	var req query.RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_body", "invalid JSON body")
		return
	}

	res, err := s.query.Run(r.Context(), caller.ID, caller.ID, id, req)
	if err != nil {
		if mapConnectionError(w, err) {
			return
		}
		writeQueryError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// ─── POST /connections/{id}/query/export?format=csv|json ─────────────────

func (s *Server) handleExportQuery(w http.ResponseWriter, r *http.Request) {
	caller, _ := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	format := query.ExportFormat(r.URL.Query().Get("format"))
	if format == "" {
		format = query.ExportCSV
	}
	if format != query.ExportCSV && format != query.ExportJSON {
		writeError(w, http.StatusBadRequest, "bad_format", "format must be csv or json")
		return
	}

	var req query.RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_body", "invalid JSON body")
		return
	}

	if err := s.query.Export(r.Context(), caller.ID, caller.ID, id, req, format, w); err != nil {
		// If we haven't written any body yet, we can still surface a JSON error.
		// Once Export starts writing rows the headers are committed and we can
		// only log.
		slog.Warn("query export error", "err", err, "conn_id", id)
		if !headersCommitted(w) {
			if mapConnectionError(w, err) {
				return
			}
			writeQueryError(w, err)
		}
	}
}

// headersCommitted reports whether http.ResponseWriter has already sent its
// headers. We can't reliably detect this from the standard interface, but our
// Export sets Content-Type before the first row, so any successful Header()
// modification afterwards is harmless.
func headersCommitted(w http.ResponseWriter) bool {
	return w.Header().Get("Content-Type") != "" && w.Header().Get("Content-Type") != "application/json"
}

// ─── GET /query-history ──────────────────────────────────────────────────

func (s *Server) handleListHistory(w http.ResponseWriter, r *http.Request) {
	caller, _ := auth.UserFromContext(r.Context())

	filters := query.HistoryFilters{
		ConnectionID: r.URL.Query().Get("connection_id"),
		Kind:         r.URL.Query().Get("kind"),
		Before:       r.URL.Query().Get("before"),
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			filters.Limit = n
		}
	}
	if sStr := r.URL.Query().Get("success"); sStr != "" {
		b := sStr == "true" || sStr == "1"
		filters.Success = &b
	}

	out, err := s.query.ListHistory(r.Context(), caller.ID, filters)
	if err != nil {
		slog.Error("list query history", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	if out == nil {
		out = []query.HistoryEntry{}
	}
	writeJSON(w, http.StatusOK, out)
}

// ─── DELETE /query-history/{id} ──────────────────────────────────────────

func (s *Server) handleDeleteHistory(w http.ResponseWriter, r *http.Request) {
	caller, _ := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	if err := s.query.DeleteHistory(r.Context(), caller.ID, id); err != nil {
		if errors.Is(err, metastore.ErrQueryHistoryNotFound) {
			writeError(w, http.StatusNotFound, "history_not_found", "history entry not found")
			return
		}
		slog.Error("delete query history", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─── GET /connections/{id}/completion?prefix=…&keyspace=… ────────────────

func (s *Server) handleCompletion(w http.ResponseWriter, r *http.Request) {
	caller, _ := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	prefix := r.URL.Query().Get("prefix")
	keyspace := r.URL.Query().Get("keyspace")

	out, err := s.query.Suggest(r.Context(), caller.ID, id, prefix, keyspace)
	if err != nil {
		if mapConnectionError(w, err) {
			return
		}
		slog.Error("completion", "err", err)
		writeError(w, http.StatusBadGateway, "cluster_unreachable", err.Error())
		return
	}
	if out == nil {
		out = []query.CompletionSuggestion{}
	}
	writeJSON(w, http.StatusOK, out)
}
