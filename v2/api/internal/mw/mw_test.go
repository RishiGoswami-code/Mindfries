package mw

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func TestWrapRecoversAndSetsRequestID(t *testing.T) {
	h := Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { panic("boom") }), quiet)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != 500 || rec.Header().Get("X-Request-Id") == "" {
		t.Fatalf("code %d id %q", rec.Code, rec.Header().Get("X-Request-Id"))
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("content type %q", ct)
	}
}

func TestRequestIDIsKeptWhenSane(t *testing.T) {
	var seen string
	h := Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { seen = RequestID(r.Context()) }), quiet)
	r := httptest.NewRequest("GET", "/x", nil)
	r.Header.Set("X-Request-Id", "abc-123")
	h.ServeHTTP(httptest.NewRecorder(), r)
	if seen != "abc-123" {
		t.Fatal(seen)
	}
	r = httptest.NewRequest("GET", "/x", nil)
	r.Header.Set("X-Request-Id", "has a space")
	h.ServeHTTP(httptest.NewRecorder(), r)
	if seen == "has a space" || seen == "" {
		t.Fatalf("an unsafe id must be replaced: %q", seen)
	}
}

func TestLimiterRefills(t *testing.T) {
	now := time.Unix(1000, 0)
	l := NewLimiter(1, 2)
	l.Now = func() time.Time { return now }
	for i := 0; i < 2; i++ {
		if ok, _ := l.Allow("k"); !ok {
			t.Fatalf("burst %d refused", i)
		}
	}
	ok, wait := l.Allow("k")
	if ok || wait <= 0 {
		t.Fatal("third request should wait")
	}
	if ok, _ := l.Allow("other"); !ok {
		t.Fatal("keys are independent")
	}
	now = now.Add(1500 * time.Millisecond)
	if ok, _ := l.Allow("k"); !ok {
		t.Fatal("a token should have refilled")
	}
}

func TestLimitAnswersWithTheContractProblem(t *testing.T) {
	l := NewLimiter(0.001, 1)
	h := Limit(l, func(*http.Request) string { return "one" }, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 429 || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("%d %v", rec.Code, rec.Header())
	}
}

func TestCORSOnlyForListedOrigins(t *testing.T) {
	h := CORS([]string{"https://portal.example"}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	for origin, want := range map[string]string{"https://portal.example": "https://portal.example", "https://evil.example": ""} {
		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set("Origin", origin)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != want {
			t.Errorf("%s: got %q want %q", origin, got, want)
		}
	}
	r := httptest.NewRequest("OPTIONS", "/", nil)
	r.Header.Set("Origin", "https://portal.example")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusNoContent {
		t.Fatal("preflight should answer 204")
	}
}
