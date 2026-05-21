package cluster

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/apache/cassandra-gocql-driver/v2"
	"github.com/ashikkabeer/cassandra-gui/internal/crypto"
	"github.com/ashikkabeer/cassandra-gui/internal/metastore"
)

// SessionFactory builds a live session from a cluster config. Real code uses
// `gocql.ClusterConfig.CreateSession`; tests inject a stub.
type SessionFactory func(ctx context.Context, cfg *gocql.ClusterConfig) (PooledSession, error)

// DefaultSessionFactory wraps gocql.CreateSession with a context timeout so
// callers can cancel a hanging connect.
func DefaultSessionFactory(ctx context.Context, cfg *gocql.ClusterConfig) (PooledSession, error) {
	type result struct {
		sess *gocql.Session
		err  error
	}
	out := make(chan result, 1)
	go func() {
		s, err := cfg.CreateSession()
		out <- result{s, err}
	}()
	select {
	case <-ctx.Done():
		go func() {
			r := <-out
			if r.sess != nil {
				r.sess.Close()
			}
		}()
		return nil, ctx.Err()
	case r := <-out:
		if r.err != nil {
			return nil, r.err
		}
		return r.sess, nil
	}
}

// ManagerOptions controls Manager behaviour.
type ManagerOptions struct {
	IdleTTL      time.Duration  // close sessions idle for longer than this
	MaxSessions  int            // hard cap; LRU-evict when exceeded
	ReapInterval time.Duration  // how often the reaper runs
	Factory      SessionFactory // how a session is built; nil → DefaultSessionFactory
}

// Manager pools live `*gocql.Session` per saved connection. Sessions are
// created on first Get, shared across all callers (gocql sessions are
// concurrency-safe), evicted when idle or when the cap is exceeded, and
// invalidated whenever the underlying connection row changes.
type Manager struct {
	cipher  *crypto.Cipher
	repo    *metastore.Connections
	factory SessionFactory
	opts    ManagerOptions

	mu       sync.Mutex
	sessions map[string]*managedSession

	stop chan struct{}
	done chan struct{}
}

// NewManager constructs a Manager and starts the background reaper goroutine.
// Call Close on shutdown to release sockets cleanly.
func NewManager(cipher *crypto.Cipher, repo *metastore.Connections, opts ManagerOptions) *Manager {
	if opts.IdleTTL <= 0 {
		opts.IdleTTL = 15 * time.Minute
	}
	if opts.MaxSessions <= 0 {
		opts.MaxSessions = 50
	}
	if opts.ReapInterval <= 0 {
		opts.ReapInterval = 5 * time.Minute
	}
	if opts.Factory == nil {
		opts.Factory = DefaultSessionFactory
	}
	m := &Manager{
		cipher:   cipher,
		repo:     repo,
		factory:  opts.Factory,
		opts:     opts,
		sessions: map[string]*managedSession{},
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	go m.reapLoop()
	return m
}

// Close stops the reaper and tears down all pooled sessions.
func (m *Manager) Close() {
	select {
	case <-m.stop:
		return // already closed
	default:
	}
	close(m.stop)
	<-m.done
	m.mu.Lock()
	for id, ms := range m.sessions {
		if ms.session != nil {
			ms.session.Close()
		}
		delete(m.sessions, id)
	}
	m.mu.Unlock()
}

// GetSession is the typed-accessor companion to Get. It calls Get and asserts
// the pooled session to *gocql.Session — what real callers (schema, query,
// dataedit services) need to run actual CQL. Returns an error if the pool
// was populated with a test stub instead of a live session, so production
// code never gets a nil *gocql.Session.
func (m *Manager) GetSession(ctx context.Context, c *metastore.Connection) (*gocql.Session, error) {
	ps, err := m.Get(ctx, c)
	if err != nil {
		return nil, err
	}
	gs, ok := ps.(*gocql.Session)
	if !ok {
		return nil, errors.New("pooled session is not a *gocql.Session (test stub?)")
	}
	return gs, nil
}

// Get returns the pooled session for the given connection, creating it on
// first call. Concurrent Get calls for the same connection wait on a single
// underlying create via sync.Once.
func (m *Manager) Get(ctx context.Context, c *metastore.Connection) (PooledSession, error) {
	if c == nil {
		return nil, errors.New("nil connection")
	}
	ms := m.entry(c.ID)
	ms.once.Do(func() {
		cfg, err := BuildClusterConfig(c, m.cipher)
		if err != nil {
			ms.err = err
			return
		}
		connectCtx := ctx
		if c.ConnectTimeoutMS > 0 {
			var cancel context.CancelFunc
			connectCtx, cancel = context.WithTimeout(ctx, time.Duration(c.ConnectTimeoutMS)*time.Millisecond)
			defer cancel()
		}
		sess, err := m.factory(connectCtx, cfg)
		if err != nil {
			ms.err = fmt.Errorf("connect: %w", err)
			return
		}
		ms.session = sess
		ms.created = time.Now()
	})
	if ms.err != nil {
		// Evict the broken entry so subsequent calls retry.
		m.invalidate(c.ID, ms.err)
		return nil, ms.err
	}
	ms.touch(time.Now())
	if m.repo != nil {
		m.repo.TouchLastUsed(context.Background(), c.ID)
	}
	return ms.session, nil
}

// TestResult is the structured outcome of a connection-test.
type TestResult struct {
	OK           bool          `json:"ok"`
	NodesUp      int           `json:"nodes_up,omitempty"`
	ClusterName  string        `json:"cluster_name,omitempty"`
	Version      string        `json:"version,omitempty"`
	Partitioner  string        `json:"partitioner,omitempty"`
	LatencyMS    int64         `json:"latency_ms,omitempty"`
	ErrorMessage string        `json:"error,omitempty"`
	Latency      time.Duration `json:"-"`
}

// Test builds a *throwaway* session against the given config, runs a tiny
// `SELECT … FROM system.local` to validate end-to-end, then closes it. The
// resulting TestResult always returns — failures are reported via OK=false +
// ErrorMessage. Never inserts anything into the pool.
func (m *Manager) Test(ctx context.Context, c *metastore.Connection) TestResult {
	start := time.Now()
	res := TestResult{}

	cfg, err := BuildClusterConfig(c, m.cipher)
	if err != nil {
		res.ErrorMessage = err.Error()
		return res
	}
	connectCtx := ctx
	if c.ConnectTimeoutMS > 0 {
		var cancel context.CancelFunc
		connectCtx, cancel = context.WithTimeout(ctx, time.Duration(c.ConnectTimeoutMS)*time.Millisecond)
		defer cancel()
	}
	sess, err := m.factory(connectCtx, cfg)
	if err != nil {
		res.ErrorMessage = err.Error()
		return res
	}
	defer sess.Close()

	// Only the real *gocql.Session can answer this; if the factory returned
	// something else (test stub), accept the connect as proof-of-life.
	if gs, ok := sess.(*gocql.Session); ok {
		var clusterName, releaseVer, partitioner string
		queryCtx := ctx
		if c.RequestTimeoutMS > 0 {
			var cancel context.CancelFunc
			queryCtx, cancel = context.WithTimeout(ctx, time.Duration(c.RequestTimeoutMS)*time.Millisecond)
			defer cancel()
		}
		if err := gs.Query(
			"SELECT cluster_name, release_version, partitioner FROM system.local",
		).WithContext(queryCtx).Scan(&clusterName, &releaseVer, &partitioner); err != nil {
			res.ErrorMessage = err.Error()
			return res
		}
		res.ClusterName = clusterName
		res.Version = releaseVer
		res.Partitioner = partitioner

		// Count nodes (best-effort).
		if iter := gs.Query("SELECT peer FROM system.peers").WithContext(queryCtx).Iter(); iter != nil {
			var peer string
			n := 1 // self
			for iter.Scan(&peer) {
				n++
			}
			_ = iter.Close()
			res.NodesUp = n
		}
	}
	res.OK = true
	res.Latency = time.Since(start)
	res.LatencyMS = res.Latency.Milliseconds()
	return res
}

// Invalidate closes and removes the pooled session for a connection. Call on
// every Update or Delete so the next Get rebuilds with current config.
func (m *Manager) Invalidate(connID string) {
	m.invalidate(connID, nil)
}

func (m *Manager) invalidate(connID string, reason error) {
	m.mu.Lock()
	ms, ok := m.sessions[connID]
	delete(m.sessions, connID)
	m.mu.Unlock()
	if !ok || ms == nil {
		return
	}
	if ms.session != nil {
		ms.session.Close()
	}
	if reason != nil {
		slog.Debug("cluster session invalidated", "conn_id", connID, "reason", reason)
	}
}

// Status reports whether a session is pooled for the given conn and when it
// was last used. Cheap; no IO.
type Status struct {
	Pooled     bool
	LastUsedAt time.Time
}

func (m *Manager) Status(connID string) Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	ms, ok := m.sessions[connID]
	if !ok || ms == nil || ms.session == nil {
		return Status{}
	}
	return Status{Pooled: true, LastUsedAt: ms.lastUsedTime()}
}

// entry returns the managedSession for connID, creating an empty placeholder
// if absent. Always evicts the LRU entry first if at cap.
func (m *Manager) entry(connID string) *managedSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	if ms, ok := m.sessions[connID]; ok {
		return ms
	}
	if len(m.sessions) >= m.opts.MaxSessions {
		m.evictOldestLocked()
	}
	ms := &managedSession{connID: connID}
	ms.touch(time.Now())
	m.sessions[connID] = ms
	return ms
}

// evictOldestLocked drops the LRU entry; caller must hold m.mu.
func (m *Manager) evictOldestLocked() {
	var oldestID string
	var oldestAt int64
	first := true
	for id, ms := range m.sessions {
		t := ms.lastUsed.Load()
		if first || t < oldestAt {
			oldestID = id
			oldestAt = t
			first = false
		}
	}
	if oldestID == "" {
		return
	}
	if ms := m.sessions[oldestID]; ms != nil && ms.session != nil {
		ms.session.Close()
	}
	delete(m.sessions, oldestID)
}

func (m *Manager) reapLoop() {
	defer close(m.done)
	t := time.NewTicker(m.opts.ReapInterval)
	defer t.Stop()
	for {
		select {
		case <-m.stop:
			return
		case now := <-t.C:
			m.reap(now)
		}
	}
}

func (m *Manager) reap(now time.Time) {
	cutoff := now.Add(-m.opts.IdleTTL)
	var stale []*managedSession
	m.mu.Lock()
	for id, ms := range m.sessions {
		if ms.session != nil && ms.lastUsedTime().Before(cutoff) {
			stale = append(stale, ms)
			delete(m.sessions, id)
		}
	}
	m.mu.Unlock()
	for _, ms := range stale {
		ms.session.Close()
		slog.Debug("cluster session reaped (idle)", "conn_id", ms.connID, "last_used", ms.lastUsedTime())
	}
}
