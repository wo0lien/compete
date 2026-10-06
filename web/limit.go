package web

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// limiter is a fixed-window counter per client IP. IPs live only in this map,
// which is wiped every window; nothing is logged or persisted.
// ponytail: fixed window allows a 2x burst at the boundary; use x/time/rate if abused.
type limiter struct {
	mu   sync.Mutex
	hits map[string]int
	max  int
}

func newLimiter(max int, window time.Duration) *limiter {
	l := &limiter{hits: map[string]int{}, max: max}
	go func() {
		for range time.Tick(window) {
			l.mu.Lock()
			clear(l.hits)
			l.mu.Unlock()
		}
	}()
	return l
}

func (l *limiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.hits[ip]++
	return l.hits[ip] <= l.max
}

// clientIP is the TCP peer, or behind a trusted proxy (Caddy) the last
// X-Forwarded-For entry, which is the one the proxy itself appended.
func (s *Server) clientIP(r *http.Request) string {
	if s.trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			return strings.TrimSpace(parts[len(parts)-1])
		}
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}

// limited guards handlers that create accounts, log in or store data.
func (s *Server) limited(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.limit.allow(s.clientIP(r)) {
			s.message(w, r, http.StatusTooManyRequests, "Slow down", "Too many requests. Try again in a minute.")
			return
		}
		h(w, r)
	}
}
