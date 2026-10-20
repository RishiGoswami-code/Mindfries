// Package mw holds the HTTP middleware every route passes through: request
// IDs, panic recovery, structured access logs, rate limits and CORS.
package mw

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ctxKey struct{}

// RequestID returns the request's ID ("" outside a request).
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}

type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusWriter) WriteHeader(c int) {
	if w.status == 0 {
		w.status = c
	}
	w.ResponseWriter.WriteHeader(c)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

// Flush keeps Server-Sent Events working through the wrapper.
func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// Wrap gives h a request ID (honouring a sane incoming X-Request-Id), recovers
// from panics with a contract-shaped 500, and logs one line per request. The
// log carries the route and status, never a query string or a token.
func Wrap(h http.Handler, log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if len(id) == 0 || len(id) > 64 || strings.ContainsAny(id, " \r\n\t") {
			b := make([]byte, 8)
			_, _ = rand.Read(b)
			id = hex.EncodeToString(b)
		}
		rw.Header().Set("X-Request-Id", id)
		w := &statusWriter{ResponseWriter: rw}
		r = r.WithContext(context.WithValue(r.Context(), ctxKey{}, id))
		start := time.Now()
		defer func() {
			if rec := recover(); rec != nil && rec != http.ErrAbortHandler {
				log.Error("panic", "request_id", id, "panic", rec, "stack", string(debug.Stack()))
				if w.status == 0 {
					w.Header().Set("Content-Type", "application/problem+json")
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(map[string]any{
						"code": "internal", "title": "Something went wrong", "status": 500, "request_id": id,
					})
				}
			}
			log.Info("request", "request_id", id, "method", r.Method, "path", r.URL.Path,
				"status", w.status, "bytes", w.bytes, "ms", time.Since(start).Milliseconds())
		}()
		h.ServeHTTP(w, r)
	})
}

// Limiter is a token bucket per key: Burst requests at once, refilled at Rate
// per second. Idle keys are dropped, so it can't grow without bound.
type Limiter struct {
	Rate  float64
	Burst float64
	Now   func() time.Time

	mu      sync.Mutex
	buckets map[string]*bucket
	lastGC  time.Time
}

type bucket struct {
	tokens float64
	at     time.Time
}

func NewLimiter(rate float64, burst int) *Limiter {
	return &Limiter{Rate: rate, Burst: float64(burst), Now: time.Now, buckets: map[string]*bucket{}}
}

// Allow takes one token for key. When it refuses it says how long to wait.
func (l *Limiter) Allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.Now()
	if now.Sub(l.lastGC) > time.Minute {
		for k, b := range l.buckets {
			if now.Sub(b.at) > 10*time.Minute {
				delete(l.buckets, k)
			}
		}
		l.lastGC = now
	}
	b := l.buckets[key]
	if b == nil {
		b = &bucket{tokens: l.Burst, at: now}
		l.buckets[key] = b
	}
	b.tokens += now.Sub(b.at).Seconds() * l.Rate
	if b.tokens > l.Burst {
		b.tokens = l.Burst
	}
	b.at = now
	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}
	return false, time.Duration((1 - b.tokens) / l.Rate * float64(time.Second))
}

// ClientIP is the caller's address. It trusts X-Forwarded-For's first entry
// only when behind is true (the deployment sits behind a proxy).
func ClientIP(r *http.Request, behind bool) string {
	if behind {
		if f := r.Header.Get("X-Forwarded-For"); f != "" {
			return strings.TrimSpace(strings.Split(f, ",")[0])
		}
	}
	h, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return h
}

// Limit wraps h so each key (from keyFn) is limited. It answers with the
// contract's rate_limited problem and a Retry-After header.
func Limit(l *Limiter, keyFn func(*http.Request) string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ok, wait := l.Allow(keyFn(r)); !ok {
			secs := int(wait.Seconds()) + 1
			w.Header().Set("Retry-After", strconv.Itoa(secs))
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": "rate_limited", "title": "Too many requests", "status": 429, "retry_after_seconds": secs,
			})
			return
		}
		h.ServeHTTP(w, r)
	})
}

// CORS allows the portals' browser pages (the floating panel) to call the
// /v2/panel routes. Only listed origins are echoed back.
func CORS(origins []string, h http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, o := range origins {
		allowed[o] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if o := r.Header.Get("Origin"); o != "" && allowed[o] {
			w.Header().Set("Access-Control-Allow-Origin", o)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Last-Event-ID")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
			w.Header().Set("Access-Control-Max-Age", "600")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		h.ServeHTTP(w, r)
	})
}
