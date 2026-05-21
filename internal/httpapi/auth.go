package httpapi

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/ashikkabeer/cassandra-gui/internal/auth"
	"github.com/ashikkabeer/cassandra-gui/internal/metastore"
)

// userDTO is the wire shape of a user — password_hash is never exposed.
type userDTO struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	Email       string `json:"email,omitempty"`
	Role        string `json:"role"`
	IsActive    bool   `json:"is_active"`
	MustResetPW bool   `json:"must_reset_pw"`
	CreatedAt   string `json:"created_at"`
}

func toUserDTO(u *metastore.User) userDTO {
	return userDTO{
		ID:          u.ID,
		Username:    u.Username,
		Email:       nullString(u.Email),
		Role:        string(u.Role),
		IsActive:    u.IsActive,
		MustResetPW: u.MustResetPW,
		CreatedAt:   u.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func nullString(n sql.NullString) string {
	if !n.Valid {
		return ""
	}
	return n.String
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_body", "invalid JSON body")
		return
	}
	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "missing_fields", "username and password are required")
		return
	}
	u, sess, token, err := s.auth.Login(r.Context(), req.Username, req.Password, clientIPForRequest(r), r.UserAgent())
	if err != nil {
		if mapServiceError(w, err) {
			return
		}
		slog.Error("login error", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	csrf, err := s.issueAuthCookies(w, token, sess.ExpiresAt)
	if err != nil {
		slog.Error("issue csrf", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	_ = csrf
	writeJSON(w, http.StatusOK, map[string]any{"user": toUserDTO(u)})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if sess, ok := auth.SessionFromContext(r.Context()); ok {
		_ = s.auth.Logout(r.Context(), sess.ID)
	}
	s.clearAuthCookies(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	if u, ok := auth.UserFromContext(r.Context()); ok {
		writeJSON(w, http.StatusOK, map[string]any{"user": toUserDTO(u)})
		return
	}
	body := map[string]any{
		"error": errorDetail{Code: "unauthenticated", Message: "sign in required"},
	}
	if s.setup.Open() {
		body["first_run"] = true
	}
	writeJSON(w, http.StatusUnauthorized, body)
}

type changePasswordRequest struct {
	Current string `json:"current"`
	New     string `json:"new"`
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "sign in required")
		return
	}
	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_body", "invalid JSON body")
		return
	}
	if err := s.auth.ChangePassword(r.Context(), u.ID, req.Current, req.New); err != nil {
		if mapServiceError(w, err) {
			return
		}
		slog.Error("change password", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type setupRequest struct {
	Token    string `json:"token"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	if !s.setup.Open() {
		writeError(w, http.StatusConflict, "setup_closed", "an admin account already exists")
		return
	}
	var req setupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_body", "invalid JSON body")
		return
	}
	if req.Token == "" || req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "missing_fields", "token, username, password are required")
		return
	}
	u, sess, token, err := s.auth.Setup(r.Context(), req.Token, req.Username, req.Email, req.Password, clientIPForRequest(r), r.UserAgent())
	if err != nil {
		if mapServiceError(w, err) {
			return
		}
		slog.Error("setup", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	if _, err := s.issueAuthCookies(w, token, sess.ExpiresAt); err != nil {
		slog.Error("issue csrf", "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": toUserDTO(u)})
}

// issueAuthCookies sets the session + CSRF cookies on a successful auth.
func (s *Server) issueAuthCookies(w http.ResponseWriter, sessionToken string, expires time.Time) (string, error) {
	csrf, err := auth.NewCSRFToken()
	if err != nil {
		return "", err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookie,
		Value:    sessionToken,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     auth.CSRFCookie,
		Value:    csrf,
		Path:     "/",
		Expires:  expires,
		HttpOnly: false,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	return csrf, nil
}

func (s *Server) clearAuthCookies(w http.ResponseWriter) {
	for _, name := range []string{auth.SessionCookie, auth.CSRFCookie} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			Expires:  time.Unix(0, 0),
			HttpOnly: name == auth.SessionCookie,
			Secure:   s.cookieSecure,
			SameSite: http.SameSiteLaxMode,
		})
	}
}

// clientIPForRequest re-exports the auth package's helper so handler files don't
// need to import the auth package just for this. Kept distinct to make the
// dependency explicit.
func clientIPForRequest(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		for i, c := range xff {
			if c == ',' {
				return xff[:i]
			}
		}
		return xff
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
