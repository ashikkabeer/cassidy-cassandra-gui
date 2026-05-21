package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/ashikkabeer/cassandra-gui/internal/auth"
	"github.com/ashikkabeer/cassandra-gui/internal/metastore"
	"github.com/go-chi/chi/v5"
)

// handleDeleteSession revokes a single session by ID. The caller may revoke
// their own sessions; admins may revoke anyone's.
func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	caller, _ := auth.UserFromContext(r.Context())
	if caller == nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "sign in required")
		return
	}
	// Look up the session to check ownership.
	sess, err := s.sessions.Get(r.Context(), id)
	if err != nil {
		if mapServiceError(w, err) {
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	if sess.UserID != caller.ID && caller.Role != metastore.RoleAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "not allowed")
		return
	}
	if err := s.sessions.Delete(r.Context(), id); err != nil {
		slog.Error("delete session", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	// If the caller revoked their own current session, clear cookies.
	currentSession, _ := auth.SessionFromContext(r.Context())
	if currentSession != nil && currentSession.ID == id {
		s.clearAuthCookies(w)
	}
	w.WriteHeader(http.StatusNoContent)
}
