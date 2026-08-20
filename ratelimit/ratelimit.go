// Package ratelimit limits how much work a caller can ask the service to do.
//
// It provides a per caller token bucket, used to turn away abuse of the public
// endpoints and to slow brute force attempts against the internal API, and a
// cap on how many requests are handled at once, which is what keeps a flood
// from using up the database connections.
package ratelimit

import (
	"net"
	"net/http"
	"strings"
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
	// Key_func identifies the client a request is counted against
	Key_func func(*http.Request) string
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
		Key_func:    Client_key,
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
		client := l.Key_func(r)

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

// Client_key_behind_proxies identifies the caller of a request that arrives
// through trusted_proxy_count reverse proxies.
//
// Each proxy appends the address it received the request from to
// X-Forwarded-For, so with one trusted proxy the last entry is the caller,
// with two the entry before it, and so on. Only that many entries are trusted;
// anything the caller put in the header itself is ignored, as is the whole
// header when no proxy is configured.
func Client_key_behind_proxies(r *http.Request, trusted_proxy_count int) string {
	if trusted_proxy_count < 1 {
		return Client_key(r)
	}

	forwarded_for := r.Header.Get("X-Forwarded-For")
	if forwarded_for == "" {
		return Client_key(r)
	}

	addresses := strings.Split(forwarded_for, ",")

	index := len(addresses) - trusted_proxy_count
	if index < 0 {
		// fewer entries than expected, so the first is the closest to the
		// caller that can be relied on
		index = 0
	}

	address := strings.TrimSpace(addresses[index])

	// an entry that is not an address is not used, so that a header value
	// cannot invent limiter keys
	if net.ParseIP(address) == nil {
		return Client_key(r)
	}

	return address
}

// Key_func_for_proxies returns a limiter key function for a service behind
// trusted_proxy_count reverse proxies.
func Key_func_for_proxies(trusted_proxy_count int) func(*http.Request) string {
	return func(r *http.Request) string {
		return Client_key_behind_proxies(r, trusted_proxy_count)
	}
}
