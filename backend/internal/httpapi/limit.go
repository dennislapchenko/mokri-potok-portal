package httpapi

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// A village-sized rate limiter: a map in memory, cleared by a restart. It
// exists so a six-digit pairing code cannot be guessed by a script, not to
// survive a serious attacker.
type limiter struct {
	mu   sync.Mutex
	hits map[string][]time.Time
}

func newLimiter() *limiter { return &limiter{hits: map[string][]time.Time{}} }

func (l *limiter) allow(key string, max int, window time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	kept := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if now.Sub(t) < window {
			kept = append(kept, t)
		}
	}
	if len(l.hits) > 5000 { // never grow without bound
		l.hits = map[string][]time.Time{}
	}
	l.hits[key] = append(kept, now)
	return len(kept) < max
}

// counter counts wrong codes across the village since the last reset.
type counter struct {
	mu sync.Mutex
	n  int
}

func (c *counter) count(by int) int { c.mu.Lock(); defer c.mu.Unlock(); c.n += by; return c.n }
func (c *counter) reset()           { c.mu.Lock(); defer c.mu.Unlock(); c.n = 0 }

// clientIP prefers the address Caddy forwards, since the backend only ever
// sees the proxy otherwise.
func clientIP(r *http.Request) string {
	if f := r.Header.Get("X-Forwarded-For"); f != "" {
		return strings.TrimSpace(strings.Split(f, ",")[0])
	}
	if h, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return h
	}
	return r.RemoteAddr
}
