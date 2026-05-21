package auth

import (
	"context"
	"net/http"

	"github.com/ashikkabeer/cassandra-gui/internal/metastore"
)

type ctxKey int

const (
	ctxUser ctxKey = iota
	ctxSession
)

const SessionCookie = "cassidy_session"

// SessionMiddleware reads the session cookie, looks up the corresponding user,
// and attaches both to the request context. It does NOT enforce auth — handlers
// that require auth should call RequireUser.
func SessionMiddleware(svc *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(SessionCookie)
			if err == nil && cookie.Value != "" {
				u, sess, err := svc.CurrentUser(r.Context(), cookie.Value)
				if err == nil {
					ctx := context.WithValue(r.Context(), ctxUser, u)
					ctx = context.WithValue(ctx, ctxSession, sess)
					r = r.WithContext(ctx)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// UserFromContext returns the authenticated user, if any.
func UserFromContext(ctx context.Context) (*metastore.User, bool) {
	u, ok := ctx.Value(ctxUser).(*metastore.User)
	return u, ok
}

// SessionFromContext returns the active session, if any.
func SessionFromContext(ctx context.Context) (*metastore.Session, bool) {
	s, ok := ctx.Value(ctxSession).(*metastore.Session)
	return s, ok
}

// RequireUser is a middleware that 401s if no session is attached.
func RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := UserFromContext(r.Context()); !ok {
			writeUnauthorized(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAdmin requires an authenticated admin.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := UserFromContext(r.Context())
		if !ok {
			writeUnauthorized(w)
			return
		}
		if u.Role != metastore.RoleAdmin {
			http.Error(w, `{"error":{"code":"forbidden","message":"admin role required"}}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":{"code":"unauthenticated","message":"sign in required"}}`))
}
