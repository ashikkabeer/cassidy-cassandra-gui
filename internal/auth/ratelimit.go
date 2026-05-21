package auth

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// IPRateLimiter implements a per-IP token bucket. Buckets are evicted after
// `idleTTL` of inactivity to bound memory.
type IPRateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*ipEntry
	r        rate.Limit
	b        int
	idleTTL  time.Duration
}

type ipEntry struct {
	limiter *rate.Limiter
	seen    time.Time
}

// NewIPRateLimiter — `attempts` over `window` (so r = attempts/window).
func NewIPRateLimiter(attempts int, window time.Duration) *IPRateLimiter {
	return &IPRateLimiter{
		limiters: map[string]*ipEntry{},
		r:        rate.Every(window / time.Duration(attempts)),
		b:        attempts,
		idleTTL:  10 * time.Minute,
	}
}

func (l *IPRateLimiter) get(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if e, ok := l.limiters[ip]; ok {
		e.seen = now
		return e.limiter
	}
	if len(l.limiters) > 0 && len(l.limiters)%64 == 0 {
		l.evictLocked(now)
	}
	lim := rate.NewLimiter(l.r, l.b)
	l.limiters[ip] = &ipEntry{limiter: lim, seen: now}
	return lim
}

func (l *IPRateLimiter) evictLocked(now time.Time) {
	for ip, e := range l.limiters {
		if now.Sub(e.seen) > l.idleTTL {
			delete(l.limiters, ip)
		}
	}
}

// Middleware applies the limiter to requests; on rate-limit, returns 429.
func (l *IPRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !l.get(ip).Allow() {
			w.Header().Set("Retry-After", "60")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"code":"rate_limited","message":"too many attempts — try again shortly"}}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// First IP in the list.
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
