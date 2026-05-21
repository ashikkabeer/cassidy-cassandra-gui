package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
)

const (
	CSRFCookie = "cassidy_csrf"
	CSRFHeader = "X-CSRF-Token"
)

// NewCSRFToken generates a 24-byte URL-safe token.
func NewCSRFToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// CSRFMiddleware implements double-submit-cookie CSRF protection. For any
// non-safe HTTP method, the request must:
//  1. carry a cassidy_csrf cookie, and
//  2. echo the same value in the X-CSRF-Token header.
//
// The cookie is set on login/setup; the SPA reads it from document.cookie and
// attaches the header automatically (it's a non-HttpOnly cookie for that reason).
//
// Safe methods (GET/HEAD/OPTIONS) skip the check. Public auth endpoints that
// don't yet have a session (POST /auth/login, /auth/setup) can be exempted by
// listing them in `exemptPaths`.
func CSRFMiddleware(exemptPaths map[string]bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isSafe(r.Method) || exemptPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}
			c, err := r.Cookie(CSRFCookie)
			if err != nil || c.Value == "" {
				csrfFail(w, "csrf_cookie_missing")
				return
			}
			h := r.Header.Get(CSRFHeader)
			if h == "" {
				csrfFail(w, "csrf_header_missing")
				return
			}
			if subtle.ConstantTimeCompare([]byte(c.Value), []byte(h)) != 1 {
				csrfFail(w, "csrf_mismatch")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isSafe(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}

func csrfFail(w http.ResponseWriter, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`{"error":{"code":"` + code + `","message":"CSRF check failed"}}`))
}
