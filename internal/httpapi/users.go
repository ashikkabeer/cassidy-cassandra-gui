package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/ashikkabeer/cassandra-gui/internal/auth"
	"github.com/ashikkabeer/cassandra-gui/internal/crypto"
	"github.com/ashikkabeer/cassandra-gui/internal/metastore"
	"github.com/go-chi/chi/v5"
)

type inviteRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	Password string `json:"password,omitempty"` // optional — if omitted, server generates
}

type inviteResponse struct {
	User         userDTO `json:"user"`
	TempPassword string  `json:"temp_password,omitempty"`
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.users.List(r.Context())
	if err != nil {
		slog.Error("list users", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	out := make([]userDTO, 0, len(users))
	for i := range users {
		out = append(out, toUserDTO(&users[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	caller, _ := auth.UserFromContext(r.Context())
	var req inviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_body", "invalid JSON body")
		return
	}
	role := metastore.Role(req.Role)
	if !role.IsValid() {
		writeError(w, http.StatusBadRequest, "invalid_role", "role must be admin, editor, or viewer")
		return
	}
	if req.Username == "" {
		writeError(w, http.StatusBadRequest, "missing_fields", "username is required")
		return
	}
	temp := req.Password
	if temp == "" {
		t, err := generateTempPassword()
		if err != nil {
			slog.Error("generate temp pw", "err", err)
			writeError(w, http.StatusInternalServerError, "internal", "internal error")
			return
		}
		temp = t
	}
	hash, err := crypto.HashPassword(temp)
	if err != nil {
		slog.Error("hash password", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	u, err := s.users.Create(r.Context(), metastore.CreateUserParams{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hash,
		Role:         role,
		MustResetPW:  req.Password == "", // server-generated → user must reset
		CreatedByID:  callerID(caller),
	})
	if err != nil {
		// Uniqueness violation surfaces as a generic error from SQLite.
		writeError(w, http.StatusConflict, "create_failed", err.Error())
		return
	}
	resp := inviteResponse{User: toUserDTO(u)}
	if req.Password == "" {
		resp.TempPassword = temp
	}
	writeJSON(w, http.StatusCreated, resp)
}

type updateUserRequest struct {
	Email    *string `json:"email,omitempty"`
	Role     *string `json:"role,omitempty"`
	IsActive *bool   `json:"is_active,omitempty"`
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req updateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_body", "invalid JSON body")
		return
	}
	params := metastore.UpdateUserParams{Email: req.Email, IsActive: req.IsActive}
	if req.Role != nil {
		role := metastore.Role(*req.Role)
		if !role.IsValid() {
			writeError(w, http.StatusBadRequest, "invalid_role", "role must be admin, editor, or viewer")
			return
		}
		params.Role = &role
	}
	if err := s.users.Update(r.Context(), id, params); err != nil {
		if mapServiceError(w, err) {
			return
		}
		slog.Error("update user", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	u, err := s.users.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	if req.IsActive != nil && !*req.IsActive {
		_ = s.sessions.DeleteByUser(r.Context(), id)
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": toUserDTO(u)})
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	// Soft-delete: deactivate + revoke sessions.
	inactive := false
	if err := s.users.Update(r.Context(), id, metastore.UpdateUserParams{IsActive: &inactive}); err != nil {
		if mapServiceError(w, err) {
			return
		}
		slog.Error("deactivate user", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	_ = s.sessions.DeleteByUser(r.Context(), id)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	temp, err := s.auth.ResetPassword(r.Context(), id)
	if err != nil {
		if mapServiceError(w, err) {
			return
		}
		slog.Error("reset password", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"temp_password": temp})
}

type sessionDTO struct {
	ID         string `json:"id"`
	CreatedAt  string `json:"created_at"`
	LastSeenAt string `json:"last_seen_at"`
	ExpiresAt  string `json:"expires_at"`
	IPAddress  string `json:"ip_address,omitempty"`
	UserAgent  string `json:"user_agent,omitempty"`
	IsCurrent  bool   `json:"is_current,omitempty"`
}

func (s *Server) handleUserSessions(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	caller, _ := auth.UserFromContext(r.Context())
	currentSession, _ := auth.SessionFromContext(r.Context())
	// Allow self or admin.
	if caller != nil && caller.ID != id && caller.Role != metastore.RoleAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "not allowed")
		return
	}
	list, err := s.sessions.ListByUser(r.Context(), id)
	if err != nil {
		slog.Error("list sessions", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	out := make([]sessionDTO, 0, len(list))
	for _, sess := range list {
		dto := sessionDTO{
			ID:         sess.ID,
			CreatedAt:  sess.CreatedAt.UTC().Format(time.RFC3339),
			LastSeenAt: sess.LastSeenAt.UTC().Format(time.RFC3339),
			ExpiresAt:  sess.ExpiresAt.UTC().Format(time.RFC3339),
			IPAddress:  nullString(sess.IPAddress),
			UserAgent:  nullString(sess.UserAgent),
		}
		if currentSession != nil && currentSession.ID == sess.ID {
			dto.IsCurrent = true
		}
		out = append(out, dto)
	}
	writeJSON(w, http.StatusOK, out)
}

func generateTempPassword() (string, error) {
	t, err := auth.NewCSRFToken() // reuse the same random helper
	if err != nil {
		return "", err
	}
	return "Cs!" + t[:14], nil
}

func callerID(u *metastore.User) string {
	if u == nil {
		return ""
	}
	return u.ID
}
