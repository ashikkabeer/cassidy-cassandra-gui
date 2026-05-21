package main

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ashikkabeer/cassandra-gui/internal/auth"
	"github.com/ashikkabeer/cassandra-gui/internal/cluster"
	"github.com/ashikkabeer/cassandra-gui/internal/config"
	"github.com/ashikkabeer/cassandra-gui/internal/connections"
	"github.com/ashikkabeer/cassandra-gui/internal/crypto"
	"github.com/ashikkabeer/cassandra-gui/internal/dataedit"
	"github.com/ashikkabeer/cassandra-gui/internal/httpapi"
	"github.com/ashikkabeer/cassandra-gui/internal/metastore"
	"github.com/ashikkabeer/cassandra-gui/internal/query"
	"github.com/ashikkabeer/cassandra-gui/internal/schema"
	"github.com/ashikkabeer/cassandra-gui/web/webfs"
)

const version = "0.0.1-dev"

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// SQLite — open + migrate.
	db, err := metastore.Open(ctx, cfg.DBPath())
	if err != nil {
		slog.Error("open db", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	users := metastore.NewUsers(db)
	sessions := metastore.NewSessions(db)
	connectionsRepo := metastore.NewConnections(db)

	// Master key + cipher for encrypting Cassandra credentials at rest.
	masterKey, err := crypto.LoadOrCreateMasterKey(cfg.MasterKeyB64, cfg.MasterKeyPath())
	if err != nil {
		slog.Error("master key", "err", err)
		os.Exit(1)
	}
	cipher, err := crypto.NewCipher(masterKey)
	if err != nil {
		slog.Error("init cipher", "err", err)
		os.Exit(1)
	}

	connectionsSvc := connections.NewService(connectionsRepo, cipher)
	clusterMgr := cluster.NewManager(cipher, connectionsRepo, cluster.ManagerOptions{})
	defer clusterMgr.Close()
	schemaSvc := schema.NewService(clusterMgr, connectionsSvc)
	historyRepo := metastore.NewQueryHistory(db)
	querySvc := query.NewService(connectionsSvc, clusterMgr, schemaSvc, historyRepo, 5000)
	dataEditSvc := dataedit.NewService(connectionsSvc, clusterMgr, schemaSvc, historyRepo)

	// First-run setup token.
	adminCount, err := users.CountAdmins(ctx)
	if err != nil {
		slog.Error("count admins", "err", err)
		os.Exit(1)
	}
	setupTok, err := auth.LoadOrCreateSetupToken(adminCount == 0, cfg.SetupToken, cfg.SetupTokenPath())
	if err != nil {
		slog.Error("setup token", "err", err)
		os.Exit(1)
	}

	authSvc := auth.NewService(users, sessions, setupTok, cfg.SessionTTL)
	loginLimiter := auth.NewIPRateLimiter(cfg.LoginRateLimit, cfg.LoginRateWindow)

	// Embedded SPA — sub-FS to drop the "dist/" prefix.
	spaSub, err := fs.Sub(webfs.FS, "dist")
	if err != nil {
		slog.Error("embed spa", "err", err)
		os.Exit(1)
	}

	srvDeps := httpapi.NewServer(httpapi.ServerOptions{
		Auth:         authSvc,
		Setup:        setupTok,
		Users:        users,
		Sessions:     sessions,
		Connections:  connectionsSvc,
		Cluster:      clusterMgr,
		Schema:       schemaSvc,
		Query:        querySvc,
		DataEdit:     dataEditSvc,
		LoginLimiter: loginLimiter,
		CookieSecure: cfg.CookieSecure,
		SPAFS:        spaSub,
		Version:      version,
	})

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           srvDeps.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Expired-session reaper.
	go reapExpiredSessions(ctx, sessions)

	// Start.
	errCh := make(chan error, 1)
	go func() {
		slog.Info("cassidy listening", "addr", cfg.ListenAddr, "data_dir", cfg.DataDir)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		slog.Info("shutting down")
	case err := <-errCh:
		slog.Error("server error", "err", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown", "err", err)
	}
}

func reapExpiredSessions(ctx context.Context, sessions *metastore.Sessions) {
	t := time.NewTicker(1 * time.Hour)
	defer t.Stop()
	// One sweep at boot.
	if n, err := sessions.DeleteExpired(context.Background()); err != nil {
		slog.Warn("session sweep", "err", err)
	} else if n > 0 {
		slog.Info("expired sessions removed", "count", n)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if n, err := sessions.DeleteExpired(context.Background()); err != nil {
				slog.Warn("session sweep", "err", err)
			} else if n > 0 {
				slog.Info("expired sessions removed", "count", n)
			}
		}
	}
}
