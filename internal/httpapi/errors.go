// Package httpapi exposes Cassidy's REST surface over chi.
package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/ashikkabeer/cassandra-gui/internal/auth"
	"github.com/ashikkabeer/cassandra-gui/internal/metastore"
)

// errorBody is the uniform JSON error envelope returned by every handler.
type errorBody struct {
	Error errorDetail `json:"error"`
}
type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("encode json", "err", err)
	}
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, errorBody{Error: errorDetail{Code: code, Message: msg}})
}

// mapServiceError maps known service errors to JSON HTTP responses.
// Returns true if it handled the error; false to let the caller decide.
func mapServiceError(w http.ResponseWriter, err error) bool {
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid username or password")
	case errors.Is(err, auth.ErrInactiveAccount):
		writeError(w, http.StatusForbidden, "inactive_account", "account is disabled")
	case errors.Is(err, auth.ErrPasswordTooShort):
		writeError(w, http.StatusBadRequest, "password_too_short", "password must be at least 12 characters")
	case errors.Is(err, auth.ErrCurrentPwWrong):
		writeError(w, http.StatusBadRequest, "current_password_wrong", "current password is incorrect")
	case errors.Is(err, auth.ErrInvalidSetupToken):
		writeError(w, http.StatusForbidden, "invalid_setup_token", "setup token is invalid")
	case errors.Is(err, auth.ErrSetupClosed):
		writeError(w, http.StatusConflict, "setup_closed", "an admin account already exists")
	case errors.Is(err, metastore.ErrUserNotFound):
		writeError(w, http.StatusNotFound, "user_not_found", "user not found")
	case errors.Is(err, metastore.ErrSessionNotFound):
		writeError(w, http.StatusNotFound, "session_not_found", "session not found")
	default:
		return false
	}
	return true
}
