package middleware

import (
	"net/http"
	"sync"
	"time"

	"workshop/internal/httpx"
)

// limiter is a sliding-window log of request times per client key. It is
// intentionally in-process: the API is a single service and the limits are
// anti-brute-force guards, not billing. A sliding window (rather than a fixed
// bucket) means a client cannot send a full allowance just before a window
// boundary and another full allowance just after it.
type limiter struct {
	mu        sync.Mutex
	limit     int
	window    time.Duration
	hits      map[string][]time.Time
	lastSweep time.Time
}

func newLimiter(limit int, window time.Duration) *limiter {
	return &limiter{limit: limit, window: window, hits: make(map[string][]time.Time)}
}

// allow records one hit for key and reports whether it is within the limit. The
// key's older timestamps are pruned on every call; the whole map is swept once
// per window so keys for clients that stopped sending requests do not
// accumulate forever.
func (l *limiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := now.Add(-l.window)
	l.sweep(cutoff, now)

	recent := pruneBefore(l.hits[key], cutoff)
	if len(recent) >= l.limit {
		l.hits[key] = recent
		return false
	}
	l.hits[key] = append(recent, now)
	return true
}

// sweep drops every timestamp that fell out of the window and removes keys that
// have no hits left. It runs at most once per window to keep the per-request
// cost independent of the number of tracked clients.
func (l *limiter) sweep(cutoff, now time.Time) {
	if !l.lastSweep.IsZero() && now.Sub(l.lastSweep) < l.window {
		return
	}
	for k, ts := range l.hits {
		ts = pruneBefore(ts, cutoff)
		if len(ts) == 0 {
			delete(l.hits, k)
			continue
		}
		l.hits[k] = ts
	}
	l.lastSweep = now
}

// pruneBefore returns the timestamps strictly after cutoff, keeping the slice
// ordered and reusing its backing array.
func pruneBefore(ts []time.Time, cutoff time.Time) []time.Time {
	i := 0
	for i < len(ts) && !ts[i].After(cutoff) {
		i++
	}
	return ts[i:]
}

// RateLimit returns a middleware that admits at most limit requests per client
// IP within window and answers 429 once exceeded. The check runs before next,
// so a denied request never reaches a credential check or a lookup.
func RateLimit(limit int, window time.Duration) func(http.Handler) http.Handler {
	l := newLimiter(limit, window)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !l.allow(httpx.ClientIP(r), time.Now()) {
				httpx.WriteError(w, http.StatusTooManyRequests, httpx.CodeTooManyRequests,
					"Zu viele Anfragen. Bitte versuchen Sie es in einer Minute erneut.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
