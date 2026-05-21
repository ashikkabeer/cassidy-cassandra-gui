package httpapi

import (
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/ashikkabeer/cassandra-gui/internal/auth"
	"github.com/ashikkabeer/cassandra-gui/internal/cluster"
	"github.com/ashikkabeer/cassandra-gui/internal/connections"
	"github.com/ashikkabeer/cassandra-gui/internal/dataedit"
	"github.com/ashikkabeer/cassandra-gui/internal/metastore"
	"github.com/ashikkabeer/cassandra-gui/internal/query"
	"github.com/ashikkabeer/cassandra-gui/internal/schema"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Server bundles all dependencies the HTTP handlers need.
type Server struct {
	auth         *auth.Service
	setup        *auth.SetupToken
	users        *metastore.Users
	sessions     *metastore.Sessions
	connections  *connections.Service
	cluster      *cluster.Manager
	schema       *schema.Service
	query        *query.Service
	dataEdit     *dataedit.Service
	loginLimiter *auth.IPRateLimiter
	csrfMW       func(http.Handler) http.Handler
	sessionMW    func(http.Handler) http.Handler

	cookieSecure bool
	spaFS        fs.FS

	version string
}

type ServerOptions struct {
	Auth         *auth.Service
	Setup        *auth.SetupToken
	Users        *metastore.Users
	Sessions     *metastore.Sessions
	Connections  *connections.Service
	Cluster      *cluster.Manager
	Schema       *schema.Service
	Query        *query.Service
	DataEdit     *dataedit.Service
	LoginLimiter *auth.IPRateLimiter
	CookieSecure bool
	SPAFS        fs.FS
	Version      string
}

func NewServer(opts ServerOptions) *Server {
	csrfExempt := map[string]bool{
		"/api/v1/auth/login": true,
		"/api/v1/auth/setup": true,
	}
	return &Server{
		auth:         opts.Auth,
		setup:        opts.Setup,
		users:        opts.Users,
		sessions:     opts.Sessions,
		connections:  opts.Connections,
		cluster:      opts.Cluster,
		schema:       opts.Schema,
		query:        opts.Query,
		dataEdit:     opts.DataEdit,
		loginLimiter: opts.LoginLimiter,
		csrfMW:       auth.CSRFMiddleware(csrfExempt),
		sessionMW:    auth.SessionMiddleware(opts.Auth),
		cookieSecure: opts.CookieSecure,
		spaFS:        opts.SPAFS,
		version:      opts.Version,
	}
}

// Handler builds the chi router.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(slogRequestLogger)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get("/version", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"version": s.version})
	})

	// /api/v1 — JSON.
	r.Route("/api/v1", func(r chi.Router) {
		// Sessions are looked up on every request so handlers can read the
		// current user from context; the *required* guard is per-route below.
		r.Use(s.sessionMW)
		r.Use(s.csrfMW)

		// Public auth endpoints — login is also rate-limited.
		r.Group(func(r chi.Router) {
			r.With(s.loginLimiter.Middleware).Post("/auth/login", s.handleLogin)
			r.With(s.loginLimiter.Middleware).Post("/auth/setup", s.handleSetup)
			r.Get("/auth/me", s.handleMe) // returns 401 if not signed in
		})

		// Authenticated endpoints.
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireUser)
			r.Post("/auth/logout", s.handleLogout)
			r.Post("/auth/change-password", s.handleChangePassword)
			r.Get("/users/{id}/sessions", s.handleUserSessions)
			r.Delete("/sessions/{id}", s.handleDeleteSession)

			// Connections — owner-scoped CRUD + test + pool control.
			r.Get("/connections", s.handleListConnections)
			r.Post("/connections", s.handleCreateConnection)
			r.Post("/connections/test", s.handleTestUnsavedConnection)
			r.Get("/connections/{id}", s.handleGetConnection)
			r.Put("/connections/{id}", s.handleUpdateConnection)
			r.Delete("/connections/{id}", s.handleDeleteConnection)
			r.Post("/connections/{id}/test", s.handleTestSavedConnection)
			r.Post("/connections/{id}/disconnect", s.handleDisconnectConnection)
			r.Get("/connections/{id}/status", s.handleConnectionStatus)

			// Schema introspection — live system_schema reads through the pool.
			r.Get("/connections/{id}/cluster-info", s.handleClusterInfo)
			r.Get("/connections/{id}/keyspaces", s.handleListKeyspaces)
			r.Get("/connections/{id}/keyspaces/{ks}/tables", s.handleListTables)
			r.Get("/connections/{id}/keyspaces/{ks}/tables/{t}", s.handleTableDetail)
			r.Get("/connections/{id}/keyspaces/{ks}/tables/{t}/ddl", s.handleTableDDL)
			r.Get("/connections/{id}/keyspaces/{ks}/types", s.handleListTypes)

			// CQL execution + history + autocomplete.
			r.Post("/connections/{id}/query", s.handleRunQuery)
			r.Post("/connections/{id}/query/export", s.handleExportQuery)
			r.Get("/connections/{id}/completion", s.handleCompletion)
			r.Get("/query-history", s.handleListHistory)
			r.Delete("/query-history/{id}", s.handleDeleteHistory)

			// Data browse + edit (table-data tab).
			r.Get("/connections/{id}/keyspaces/{ks}/tables/{t}/rows", s.handleListRows)
			r.Post("/connections/{id}/keyspaces/{ks}/tables/{t}/rows/preview", s.handlePreviewRows)
			r.Post("/connections/{id}/keyspaces/{ks}/tables/{t}/rows/commit", s.handleCommitRows)
		})

		// Admin-only endpoints.
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireUser)
			r.Use(auth.RequireAdmin)
			r.Get("/users", s.handleListUsers)
			r.Post("/users", s.handleCreateUser)
			r.Patch("/users/{id}", s.handleUpdateUser)
			r.Delete("/users/{id}", s.handleDeleteUser)
			r.Post("/users/{id}/reset-password", s.handleResetPassword)
		})
	})

	// 404 for any /api/* not matched above (don't leak the SPA to API consumers).
	r.HandleFunc("/api/*", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "no such endpoint")
	})

	// Embedded SPA + static assets fallback.
	r.Handle("/*", s.spaHandler())

	return r
}

func (s *Server) spaHandler() http.Handler {
	if s.spaFS == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "frontend not embedded", http.StatusInternalServerError)
		})
	}
	fileServer := http.FileServer(http.FS(s.spaFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" {
			if _, err := fs.Stat(s.spaFS, path); err == nil {
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		f, err := s.spaFS.Open("index.html")
		if err != nil {
			http.Error(w, "index.html missing — run `pnpm build` in web/", http.StatusInternalServerError)
			return
		}
		defer f.Close()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.Copy(w, f)
	})
}

func slogRequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		dur := time.Since(start)
		// Skip noisy static asset logs.
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			return
		}
		slog.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"bytes", ww.BytesWritten(),
			"dur_ms", dur.Milliseconds(),
			"ip", clientIPForRequest(r),
		)
	})
}
