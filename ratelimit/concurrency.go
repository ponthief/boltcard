package ratelimit

import (
	"net/http"

	log "github.com/sirupsen/logrus"
)

// Concurrency_limiter caps how many requests are handled at once.
//
// Each request handled opens database connections of its own, so without a cap
// a flood of requests can use up the connections the database allows and stop
// the service serving anyone. Rate limiting per caller does not cover this on
// its own, as a flood can come from many addresses.
type Concurrency_limiter struct {
	slots chan struct{}
}

// New_concurrency_limiter returns a limiter allowing max_in_flight requests to
// be handled at once.
func New_concurrency_limiter(max_in_flight int) *Concurrency_limiter {
	if max_in_flight < 1 {
		max_in_flight = 1
	}

	return &Concurrency_limiter{
		slots: make(chan struct{}, max_in_flight),
	}
}

// In_flight returns how many requests are being handled.
func (c *Concurrency_limiter) In_flight() int {
	return len(c.slots)
}

// Middleware wraps a handler so that requests over the limit are turned away
// with HTTP 503 rather than queueing up and holding resources.
func (c *Concurrency_limiter) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		select {
		case c.slots <- struct{}{}:
			defer func() { <-c.slots }()
		default:
			log.WithFields(log.Fields{
				"path": r.URL.Path}).Warn("request rejected - too many requests in flight")
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status":"ERROR","reason":"service busy"}`))
			return
		}

		next(w, r)
	}
}
