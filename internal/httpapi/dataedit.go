package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/ashikkabeer/cassandra-gui/internal/auth"
	"github.com/ashikkabeer/cassandra-gui/internal/dataedit"
	"github.com/ashikkabeer/cassandra-gui/internal/schema"
	"github.com/go-chi/chi/v5"
)

// writeDataEditError maps dataedit/builder errors to the uniform JSON envelope.
func writeDataEditError(w http.ResponseWriter, err error) bool {
	switch {
	case errors.Is(err, dataedit.ErrReadOnly):
		writeError(w, http.StatusForbidden, "read_only_connection", "connection is read-only")
	case errors.Is(err, dataedit.ErrIncompletePK):
		writeError(w, http.StatusBadRequest, "incomplete_primary_key", err.Error())
	case errors.Is(err, dataedit.ErrUnsupportedColumn):
		writeError(w, http.StatusBadRequest, "unsupported_column", err.Error())
	case errors.Is(err, dataedit.ErrNoChanges):
		writeError(w, http.StatusBadRequest, "no_changes", err.Error())
	case errors.Is(err, dataedit.ErrUnknownColumn):
		writeError(w, http.StatusBadRequest, "unknown_column", err.Error())
	case errors.Is(err, dataedit.ErrInvalidPageState):
		writeError(w, http.StatusBadRequest, "invalid_page_state", err.Error())
	case errors.Is(err, schema.ErrTableNotFound):
		writeError(w, http.StatusNotFound, "table_not_found", "table not found")
	default:
		return false
	}
	return true
}

func (s *Server) handleListRows(w http.ResponseWriter, r *http.Request) {
	caller, _ := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	ks := chi.URLParam(r, "ks")
	tbl := chi.URLParam(r, "t")
	pageSize := 0
	if p := r.URL.Query().Get("page_size"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			pageSize = n
		}
	}
	page, err := s.dataEdit.Rows(r.Context(), caller.ID, id, ks, tbl, pageSize, r.URL.Query().Get("page_state"))
	if err != nil {
		if mapConnectionError(w, err) || writeDataEditError(w, err) {
			return
		}
		slog.Error("list rows", "err", err, "conn_id", id, "ks", ks, "tbl", tbl)
		writeError(w, http.StatusBadGateway, "cluster_unreachable", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) handlePreviewRows(w http.ResponseWriter, r *http.Request) {
	caller, _ := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	ks := chi.URLParam(r, "ks")
	tbl := chi.URLParam(r, "t")
	var cs dataedit.ChangeSet
	if err := json.NewDecoder(r.Body).Decode(&cs); err != nil {
		writeError(w, http.StatusBadRequest, "bad_body", "invalid JSON body")
		return
	}
	prev, err := s.dataEdit.Preview(r.Context(), caller.ID, id, ks, tbl, cs)
	if err != nil {
		if mapConnectionError(w, err) || writeDataEditError(w, err) {
			return
		}
		slog.Error("preview rows", "err", err)
		writeError(w, http.StatusBadRequest, "preview_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, prev)
}

func (s *Server) handleCommitRows(w http.ResponseWriter, r *http.Request) {
	caller, _ := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	ks := chi.URLParam(r, "ks")
	tbl := chi.URLParam(r, "t")
	var cs dataedit.ChangeSet
	if err := json.NewDecoder(r.Body).Decode(&cs); err != nil {
		writeError(w, http.StatusBadRequest, "bad_body", "invalid JSON body")
		return
	}
	res, err := s.dataEdit.Commit(r.Context(), caller.ID, caller.ID, id, ks, tbl, cs)
	if err != nil {
		if mapConnectionError(w, err) || writeDataEditError(w, err) {
			return
		}
		slog.Error("commit rows", "err", err, "conn_id", id, "ks", ks, "tbl", tbl)
		writeError(w, http.StatusBadGateway, "cluster_unreachable", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}
