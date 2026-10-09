package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// newRateLimitedHandler wires RateLimit(limit, window) around a test handler
// that records how many times it was reached.
func newRateLimitedHandler(limit int, window time.Duration) (http.Handler, *int) {
	calls := 0
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	})
	return RateLimit(limit, window)(ok), &calls
}

func requestFrom(clientIP string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.Header.Set("X-Forwarded-For", clientIP)
	return req
}

func decodeErrorCode(t *testing.T, body []byte) string {
	t.Helper()
	var envelope struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode error body %q: %v", body, err)
	}
	if envelope.Error.Code == "" || envelope.Error.Message == "" {
		t.Fatalf("error envelope incomplete: %+v", envelope.Error)
	}
	return envelope.Error.Code
}

// TestLoginLimitBlocksEleventhRequest proves AC-23: after ten requests in the
// window the eleventh from the same client answers 429 with the stable error
// body, and the handler (the credential check) is never reached for it.
func TestLoginLimitBlocksEleventhRequest(t *testing.T) {
	h, calls := newRateLimitedHandler(10, time.Minute)
	const client = "203.0.113.7"

	for i := 0; i < 10; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, requestFrom(client))
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: got %d, want 200", i+1, rec.Code)
		}
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, requestFrom(client))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("eleventh request: got %d, want 429", rec.Code)
	}
	if code := decodeErrorCode(t, rec.Body.Bytes()); code != "too_many_requests" {
		t.Fatalf("error code: got %q, want too_many_requests", code)
	}
	if *calls != 10 {
		t.Fatalf("handler reached %d times, want 10 (must stop before evaluation)", *calls)
	}
}

// TestLoginLimitIsPerClient proves one client exhausting its allowance leaves a
// second client's requests untouched.
func TestLoginLimitIsPerClient(t *testing.T) {
	h, _ := newRateLimitedHandler(10, time.Minute)

	for i := 0; i < 11; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, requestFrom("198.51.100.1"))
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, requestFrom("198.51.100.2"))
	if rec.Code != http.StatusOK {
		t.Fatalf("other client: got %d, want 200 (unaffected by first client)", rec.Code)
	}
}

// TestPublicLimitBlocksTwentyFirstRequest proves AC-25/AC-31: the public lookup
// allowance is 20 per window per client.
func TestPublicLimitBlocksTwentyFirstRequest(t *testing.T) {
	h, calls := newRateLimitedHandler(20, time.Minute)
	const client = "203.0.113.9"

	for i := 0; i < 20; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, requestFrom(client))
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: got %d, want 200", i+1, rec.Code)
		}
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, requestFrom(client))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("twenty-first request: got %d, want 429", rec.Code)
	}
	if code := decodeErrorCode(t, rec.Body.Bytes()); code != "too_many_requests" {
		t.Fatalf("error code: got %q, want too_many_requests", code)
	}
	if *calls != 20 {
		t.Fatalf("handler reached %d times, want 20", *calls)
	}
}

// TestSlidingWindowReleasesAfterWindow proves the window truly slides: once the
// oldest hits fall out, the client is admitted again.
func TestSlidingWindowReleasesAfterWindow(t *testing.T) {
	l := newLimiter(2, time.Minute)
	start := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)

	if !l.allow("a", start) || !l.allow("a", start.Add(10*time.Second)) {
		t.Fatal("first two requests within the limit must be allowed")
	}
	if l.allow("a", start.Add(20*time.Second)) {
		t.Fatal("third request inside the window must be denied")
	}
	if !l.allow("a", start.Add(time.Minute+5*time.Second)) {
		t.Fatal("request after the window must be allowed again")
	}
}

// TestLimiterCleanupDropsStaleKeys proves the in-process map does not grow
// forever: keys with no hits left in the window are removed by the sweep.
func TestLimiterCleanupDropsStaleKeys(t *testing.T) {
	l := newLimiter(5, time.Minute)
	start := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)

	l.allow("stale", start)
	l.allow("fresh", start.Add(2*time.Minute))

	if _, ok := l.hits["stale"]; ok {
		t.Fatalf("stale key should have been swept, map = %v", l.hits)
	}
	if _, ok := l.hits["fresh"]; !ok {
		t.Fatalf("fresh key must remain, map = %v", l.hits)
	}
}
