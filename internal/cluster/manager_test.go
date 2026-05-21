package cluster

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/apache/cassandra-gocql-driver/v2"
	"github.com/ashikkabeer/cassandra-gui/internal/metastore"
)

// fakeSession satisfies PooledSession.
type fakeSession struct {
	id     string
	closed atomic.Bool
}

func (f *fakeSession) Close() { f.closed.Store(true) }

func newTestManager(t *testing.T, factory SessionFactory, opts ManagerOptions) *Manager {
	t.Helper()
	opts.Factory = factory
	m := NewManager(nil, nil, opts)
	t.Cleanup(m.Close)
	return m
}

func sampleConn(id string) *metastore.Connection {
	return &metastore.Connection{
		ID:               id,
		Name:             "c-" + id,
		Hosts:            []string{"127.0.0.1"},
		Port:             9042,
		Consistency:      "ONE",
		ConnectTimeoutMS: 1000,
		RequestTimeoutMS: 1000,
	}
}

func TestManagerLazyCreateAndReuse(t *testing.T) {
	var calls atomic.Int32
	factory := func(ctx context.Context, cfg *gocql.ClusterConfig) (PooledSession, error) {
		calls.Add(1)
		return &fakeSession{id: cfg.Hosts[0]}, nil
	}
	m := newTestManager(t, factory, ManagerOptions{IdleTTL: time.Hour, MaxSessions: 10, ReapInterval: time.Hour})

	conn := sampleConn("a")
	ctx := context.Background()
	s1, err := m.Get(ctx, conn)
	if err != nil {
		t.Fatalf("get 1: %v", err)
	}
	s2, err := m.Get(ctx, conn)
	if err != nil {
		t.Fatalf("get 2: %v", err)
	}
	if s1 != s2 {
		t.Fatal("expected same session on second Get")
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected 1 factory call, got %d", got)
	}
}

func TestManagerConcurrentFirstCreateDeduped(t *testing.T) {
	var calls atomic.Int32
	gate := make(chan struct{})
	factory := func(ctx context.Context, cfg *gocql.ClusterConfig) (PooledSession, error) {
		calls.Add(1)
		<-gate // block so multiple goroutines hit the once.Do simultaneously
		return &fakeSession{id: cfg.Hosts[0]}, nil
	}
	m := newTestManager(t, factory, ManagerOptions{IdleTTL: time.Hour, MaxSessions: 10, ReapInterval: time.Hour})

	conn := sampleConn("a")
	var wg sync.WaitGroup
	wg.Add(5)
	sessCh := make(chan PooledSession, 5)
	for i := 0; i < 5; i++ {
		go func() {
			defer wg.Done()
			s, err := m.Get(context.Background(), conn)
			if err != nil {
				t.Errorf("get: %v", err)
				return
			}
			sessCh <- s
		}()
	}
	// Give goroutines a moment to all reach once.Do.
	time.Sleep(50 * time.Millisecond)
	close(gate)
	wg.Wait()
	close(sessCh)

	if got := calls.Load(); got != 1 {
		t.Fatalf("expected exactly 1 factory call, got %d", got)
	}
	var first PooledSession
	for s := range sessCh {
		if first == nil {
			first = s
		} else if s != first {
			t.Fatal("different sessions returned to concurrent Gets")
		}
	}
}

func TestManagerInvalidateClosesAndEvicts(t *testing.T) {
	factory := func(ctx context.Context, cfg *gocql.ClusterConfig) (PooledSession, error) {
		return &fakeSession{id: cfg.Hosts[0]}, nil
	}
	m := newTestManager(t, factory, ManagerOptions{IdleTTL: time.Hour, MaxSessions: 10, ReapInterval: time.Hour})

	conn := sampleConn("a")
	s, _ := m.Get(context.Background(), conn)
	fake := s.(*fakeSession)
	m.Invalidate(conn.ID)
	if !fake.closed.Load() {
		t.Fatal("expected Invalidate to close the pooled session")
	}
	st := m.Status(conn.ID)
	if st.Pooled {
		t.Fatal("expected status to report not-pooled after Invalidate")
	}
}

func TestManagerIdleReaper(t *testing.T) {
	factory := func(ctx context.Context, cfg *gocql.ClusterConfig) (PooledSession, error) {
		return &fakeSession{id: cfg.Hosts[0]}, nil
	}
	m := newTestManager(t, factory, ManagerOptions{IdleTTL: 10 * time.Millisecond, MaxSessions: 10, ReapInterval: time.Hour})

	conn := sampleConn("a")
	s, _ := m.Get(context.Background(), conn)
	fake := s.(*fakeSession)
	// Force a reap with a "now" past the TTL — easier than waiting on the ticker.
	time.Sleep(15 * time.Millisecond)
	m.reap(time.Now())
	if !fake.closed.Load() {
		t.Fatal("expected idle session to be reaped + closed")
	}
	if m.Status(conn.ID).Pooled {
		t.Fatal("expected status not-pooled after reap")
	}
}

func TestManagerLRUEviction(t *testing.T) {
	factory := func(ctx context.Context, cfg *gocql.ClusterConfig) (PooledSession, error) {
		return &fakeSession{id: cfg.Hosts[0]}, nil
	}
	m := newTestManager(t, factory, ManagerOptions{IdleTTL: time.Hour, MaxSessions: 2, ReapInterval: time.Hour})

	a, _ := m.Get(context.Background(), sampleConn("a"))
	time.Sleep(2 * time.Millisecond) // ensure lastUsed distinct
	b, _ := m.Get(context.Background(), sampleConn("b"))
	time.Sleep(2 * time.Millisecond)
	// Touching `b` keeps it newer than `a`.
	_, _ = m.Get(context.Background(), sampleConn("b"))
	time.Sleep(2 * time.Millisecond)
	// Adding `c` should evict `a` (oldest lastUsed).
	c, _ := m.Get(context.Background(), sampleConn("c"))

	if !a.(*fakeSession).closed.Load() {
		t.Fatal("expected `a` to be evicted on overflow")
	}
	if b.(*fakeSession).closed.Load() || c.(*fakeSession).closed.Load() {
		t.Fatal("`b` and `c` should still be live")
	}
}

func TestManagerCreateErrorRetries(t *testing.T) {
	var attempts atomic.Int32
	factory := func(ctx context.Context, cfg *gocql.ClusterConfig) (PooledSession, error) {
		n := attempts.Add(1)
		if n == 1 {
			return nil, errors.New("simulated connect failure")
		}
		return &fakeSession{id: cfg.Hosts[0]}, nil
	}
	m := newTestManager(t, factory, ManagerOptions{IdleTTL: time.Hour, MaxSessions: 10, ReapInterval: time.Hour})

	conn := sampleConn("a")
	if _, err := m.Get(context.Background(), conn); err == nil {
		t.Fatal("expected first Get to surface the error")
	}
	// Failed entries should be evicted so the next call retries.
	if _, err := m.Get(context.Background(), conn); err != nil {
		t.Fatalf("expected retry to succeed, got %v", err)
	}
	if attempts.Load() != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts.Load())
	}
}
