package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/ashikkabeer/cassandra-gui/internal/auth"
	"github.com/ashikkabeer/cassandra-gui/internal/schema"
	"github.com/go-chi/chi/v5"
)

func (s *Server) handleClusterInfo(w http.ResponseWriter, r *http.Request) {
	caller, _ := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	info, err := s.schema.ClusterInfo(r.Context(), caller.ID, id)
	if err != nil {
		if mapConnectionError(w, err) {
			return
		}
		slog.Error("cluster info", "err", err, "conn_id", id)
		writeError(w, http.StatusBadGateway, "cluster_unreachable", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleListKeyspaces(w http.ResponseWriter, r *http.Request) {
	caller, _ := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	out, err := s.schema.Keyspaces(r.Context(), caller.ID, id)
	if err != nil {
		if mapConnectionError(w, err) {
			return
		}
		slog.Error("list keyspaces", "err", err, "conn_id", id)
		writeError(w, http.StatusBadGateway, "cluster_unreachable", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleListTables(w http.ResponseWriter, r *http.Request) {
	caller, _ := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	ks := chi.URLParam(r, "ks")
	out, err := s.schema.Tables(r.Context(), caller.ID, id, ks)
	if err != nil {
		if mapConnectionError(w, err) {
			return
		}
		slog.Error("list tables", "err", err, "conn_id", id, "ks", ks)
		writeError(w, http.StatusBadGateway, "cluster_unreachable", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleTableDetail(w http.ResponseWriter, r *http.Request) {
	caller, _ := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	ks := chi.URLParam(r, "ks")
	tbl := chi.URLParam(r, "t")
	detail, err := s.schema.Table(r.Context(), caller.ID, id, ks, tbl)
	if err != nil {
		if mapConnectionError(w, err) {
			return
		}
		if errors.Is(err, schema.ErrTableNotFound) {
			writeError(w, http.StatusNotFound, "table_not_found", "table not found")
			return
		}
		slog.Error("table detail", "err", err, "conn_id", id, "ks", ks, "tbl", tbl)
		writeError(w, http.StatusBadGateway, "cluster_unreachable", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleTableDDL(w http.ResponseWriter, r *http.Request) {
	caller, _ := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	ks := chi.URLParam(r, "ks")
	tbl := chi.URLParam(r, "t")
	ddl, err := s.schema.DDL(r.Context(), caller.ID, id, ks, tbl)
	if err != nil {
		if mapConnectionError(w, err) {
			return
		}
		if errors.Is(err, schema.ErrTableNotFound) {
			writeError(w, http.StatusNotFound, "table_not_found", "table not found")
			return
		}
		slog.Error("table ddl", "err", err, "conn_id", id, "ks", ks, "tbl", tbl)
		writeError(w, http.StatusBadGateway, "cluster_unreachable", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ddl": ddl})
}

func (s *Server) handleListTypes(w http.ResponseWriter, r *http.Request) {
	caller, _ := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	ks := chi.URLParam(r, "ks")
	out, err := s.schema.Types(r.Context(), caller.ID, id, ks)
	if err != nil {
		if mapConnectionError(w, err) {
			return
		}
		slog.Error("list types", "err", err, "conn_id", id, "ks", ks)
		writeError(w, http.StatusBadGateway, "cluster_unreachable", err.Error())
		return
	}
	// Always return [] not null so the client doesn't need to defend against nil.
	if out == nil {
		out = []schema.UDT{}
	}
	writeJSON(w, http.StatusOK, out)
}
