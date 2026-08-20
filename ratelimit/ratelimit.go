// Package ratelimit provides a small in-memory per client token bucket rate
// limiter, used to slow down brute force attempts against the internal API.
package ratelimit

import (
	"net"
	"net/http"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type bucket struct {
	tokens    float64
	last_seen time.Time
}

// Limiter allows up to burst requests per client, refilling at
// requests_per_minute per minute.
type Limiter struct {
	mutex   sync.Mutex
	buckets map[string]*bucket
	// tokens added per second
	refill_rate float64
	burst       float64
	// now is a variable so that tests do not need to sleep
	now func() time.Time
}

// idle buckets are dropped after this long, to bound memory use
const bucket_expiry = 10 * time.Minute

// New returns a Limiter allowing requests_per_minute requests per client per
// minute, with up to burst requests arriving at once.
func New(requests_per_minute int, burst int) *Limiter {
	if requests_per_minute < 1 {
		requests_per_minute = 1
	}

	if burst < 1 {
		burst = 1
	}

	return &Limiter{
		buckets:     make(map[string]*bucket),
		refill_rate: float64(requests_per_minute) / 60.0,
		burst:       float64(burst),
		now:         time.Now,
	}
}

// Allow reports whether a request from the given client key may proceed.
func (l *Limiter) Allow(client_key string) bool {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	now := l.now()

	b, found := l.buckets[client_key]
	if !found {
		b = &bucket{tokens: l.burst, last_seen: now}
		l.buckets[client_key] = b
	} else {
		elapsed := now.Sub(b.last_seen).Seconds()
		if elapsed > 0 {
			b.tokens += elapsed * l.refill_rate
			if b.tokens > l.burst {
				b.tokens = l.burst
			}
		}
		b.last_seen = now
	}

	l.prune(now)

	if b.tokens < 1 {
		return false
	}

	b.tokens--

	return true
}

// prune drops buckets that have been idle long enough to have refilled.
// the caller must hold the mutex.
func (l *Limiter) prune(now time.Time) {
	for key, b := range l.buckets {
		if now.Sub(b.last_seen) > bucket_expiry {
			delete(l.buckets, key)
		}
	}
}

// Middleware wraps a handler so that requests over the limit are rejected
// with HTTP 429 instead of reaching the handler.
func (l *Limiter) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		client := Client_key(r)

		if !l.Allow(client) {
			log.WithFields(log.Fields{
				"path": r.URL.Path}).Warn("request rejected - rate limit exceeded")
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"status":"ERROR","reason":"too many requests"}`))
			return
		}

		next(w, r)
	}
}

// Client_key identifies the caller of a request by network address.
// Forwarded headers are deliberately ignored, as they are set by the caller.
func Client_key(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}
