package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/ashikkabeer/cassandra-gui/internal/auth"
	"github.com/ashikkabeer/cassandra-gui/internal/connections"
	"github.com/ashikkabeer/cassandra-gui/internal/metastore"
	"github.com/go-chi/chi/v5"
)

func (s *Server) handleListConnections(w http.ResponseWriter, r *http.Request) {
	caller, _ := auth.UserFromContext(r.Context())
	list, err := s.connections.List(r.Context(), caller.ID)
	if err != nil {
		slog.Error("list connections", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	out := make([]connections.ConnectionDTO, 0, len(list))
	for i := range list {
		out = append(out, connections.ToDTO(&list[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateConnection(w http.ResponseWriter, r *http.Request) {
	caller, _ := auth.UserFromContext(r.Context())
	var req connections.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_body", "invalid JSON body")
		return
	}
	c, err := s.connections.Create(r.Context(), caller.ID, req)
	if err != nil {
		if errors.Is(err, connections.ErrInvalidRequest) {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		slog.Error("create connection", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, connections.ToDTO(c))
}

func (s *Server) handleGetConnection(w http.ResponseWriter, r *http.Request) {
	caller, _ := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	c, err := s.connections.Get(r.Context(), caller.ID, id)
	if err != nil {
		if mapServiceError(w, err) || mapConnectionError(w, err) {
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, connections.ToDTO(c))
}

func (s *Server) handleUpdateConnection(w http.ResponseWriter, r *http.Request) {
	caller, _ := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	var req connections.UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_body", "invalid JSON body")
		return
	}
	c, err := s.connections.Update(r.Context(), caller.ID, id, req)
	if err != nil {
		if errors.Is(err, connections.ErrInvalidRequest) {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		if mapConnectionError(w, err) {
			return
		}
		slog.Error("update connection", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	// Any change invalidates the pooled session so the next Get rebuilds, and
	// the schema cache so we don't serve stale tables under the new config.
	if s.cluster != nil {
		s.cluster.Invalidate(id)
	}
	if s.schema != nil {
		s.schema.InvalidateConn(id)
	}
	writeJSON(w, http.StatusOK, connections.ToDTO(c))
}

func (s *Server) handleDeleteConnection(w http.ResponseWriter, r *http.Request) {
	caller, _ := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	if err := s.connections.Delete(r.Context(), caller.ID, id); err != nil {
		if mapConnectionError(w, err) {
			return
		}
		slog.Error("delete connection", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	if s.cluster != nil {
		s.cluster.Invalidate(id)
	}
	if s.schema != nil {
		s.schema.InvalidateConn(id)
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleTestSavedConnection runs `cluster.Test` against an already-saved
// connection (looked up by ID).
func (s *Server) handleTestSavedConnection(w http.ResponseWriter, r *http.Request) {
	caller, _ := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	c, err := s.connections.Get(r.Context(), caller.ID, id)
	if err != nil {
		if mapConnectionError(w, err) {
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	res := s.cluster.Test(r.Context(), c)
	writeJSON(w, http.StatusOK, res)
}

// handleTestUnsavedConnection runs `cluster.Test` against an unsaved config
// supplied in the request body — used by the form's "Test connection" button
// before the user clicks Save.
func (s *Server) handleTestUnsavedConnection(w http.ResponseWriter, r *http.Request) {
	var req connections.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_body", "invalid JSON body")
		return
	}
	c, err := s.connections.BuildEphemeral(req)
	if err != nil {
		if errors.Is(err, connections.ErrInvalidRequest) {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	res := s.cluster.Test(r.Context(), c)
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleDisconnectConnection(w http.ResponseWriter, r *http.Request) {
	caller, _ := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	if _, err := s.connections.Get(r.Context(), caller.ID, id); err != nil {
		if mapConnectionError(w, err) {
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	s.cluster.Invalidate(id)
	w.WriteHeader(http.StatusNoContent)
}

type connectionStatusDTO struct {
	Pooled     bool   `json:"pooled"`
	LastUsedAt string `json:"last_used_at,omitempty"`
}

func (s *Server) handleConnectionStatus(w http.ResponseWriter, r *http.Request) {
	caller, _ := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	if _, err := s.connections.Get(r.Context(), caller.ID, id); err != nil {
		if mapConnectionError(w, err) {
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	st := s.cluster.Status(id)
	dto := connectionStatusDTO{Pooled: st.Pooled}
	if !st.LastUsedAt.IsZero() {
		dto.LastUsedAt = st.LastUsedAt.UTC().Format(time.RFC3339)
	}
	writeJSON(w, http.StatusOK, dto)
}

// mapConnectionError maps connection-layer errors to JSON HTTP responses.
// Returns true if handled.
func mapConnectionError(w http.ResponseWriter, err error) bool {
	if errors.Is(err, metastore.ErrConnectionNotFound) {
		writeError(w, http.StatusNotFound, "connection_not_found", "connection not found")
		return true
	}
	return false
}
