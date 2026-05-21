package cluster

import (
	"sync"
	"sync/atomic"
	"time"
)

// PooledSession is the minimal interface the manager needs from a session.
// gocql.Session satisfies it (Close()). Defining it as an interface lets the
// manager unit-test with a fake without spinning up a real Cassandra.
type PooledSession interface {
	Close()
}

// managedSession wraps a pooled session with the metadata needed for LRU
// eviction and concurrent-create deduplication.
type managedSession struct {
	connID   string
	session  PooledSession
	created  time.Time
	lastUsed atomic.Int64 // unix nanos; updated on every Get
	once     sync.Once
	err      error // result of the once-only create
}

func (m *managedSession) touch(now time.Time) {
	m.lastUsed.Store(now.UnixNano())
}

func (m *managedSession) lastUsedTime() time.Time {
	return time.Unix(0, m.lastUsed.Load())
}
