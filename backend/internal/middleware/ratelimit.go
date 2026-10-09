package middleware

import (
	"net/http"
	"sync"
	"time"

	"workshop/internal/httpx"
)

// limiter is a fixed-window counter per client key. It is intentionally simple
// and in-process: the API is a single service and the limits are anti-brute-force
// guards, not billing.
type limiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	hits   map[string]*windowStat
}

type windowStat struct {
	count int
	reset time.Time
}

func newLimiter(limit int, window time.Duration) *limiter {
	return &limiter{limit: limit, window: window, hits: make(map[string]*windowStat)}
}

// allow records one hit for key and reports whether it is within the limit.
func (l *limiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.hits[key]
	if !ok || now.After(entry.reset) {
		l.hits[key] = &windowStat{count: 1, reset: now.Add(l.window)}
		return true
	}
	entry.count++
	return entry.count <= l.limit
}

// RateLimit returns a middleware that admits at most limit requests per client
// IP within window and answers 429 once exceeded.
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
