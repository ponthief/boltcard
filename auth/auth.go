// Package auth provides API key authentication for the internal API.
//
// The internal API can create cards, read card settings and wipe cards.
// A caller able to create a card can set its own spending limits, read the
// card keys from the /new endpoint and then withdraw from the node, so these
// endpoints must never be reachable without a shared secret.
package auth

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"

	"github.com/boltcard/boltcard/db"
	log "github.com/sirupsen/logrus"
)

// Min_key_length is the shortest internal API key that is accepted.
// A key shorter than this (or no key at all) disables the internal API.
const Min_key_length = 16

// Key_provider returns the configured internal API key.
// It is a variable so that tests can supply a key without a database.
var Key_provider = Internal_api_key

// Internal_api_key reads the internal API key from the environment, falling
// back to the settings table. The environment is checked first so that a key
// can be supplied to a container without writing it to the database.
func Internal_api_key() string {
	key := strings.TrimSpace(os.Getenv("INTERNAL_API_KEY"))
	if key != "" {
		return key
	}

	return strings.TrimSpace(db.Get_setting("INTERNAL_API_KEY"))
}

// Key_configured reports whether a usable internal API key is set.
func Key_configured() bool {
	return len(Key_provider()) >= Min_key_length
}

// Request_token returns the API key presented by the caller.
// It is taken from an `Authorization: Bearer <key>` header or, for
// convenience with simple clients, from an `X-Api-Key` header.
// The key is never read from the URL query string, as request URLs are logged.
func Request_token(r *http.Request) string {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if header != "" {
		// the scheme name is case insensitive (RFC 7235)
		const scheme = "bearer "
		if len(header) > len(scheme) && strings.EqualFold(header[:len(scheme)], scheme) {
			return strings.TrimSpace(header[len(scheme):])
		}
		return ""
	}

	return strings.TrimSpace(r.Header.Get("X-Api-Key"))
}

// Token_valid compares a presented key with the configured key in constant
// time. An unconfigured or too short configured key is never valid, so that a
// missing setting cannot open up the internal API.
func Token_valid(configured_key string, presented_key string) bool {
	if len(configured_key) < Min_key_length {
		return false
	}

	if presented_key == "" {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(configured_key), []byte(presented_key)) == 1
}

// Require_internal_api_key wraps an internal API handler so that it only runs
// for callers presenting the configured internal API key.
func Require_internal_api_key(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		configured_key := Key_provider()

		if len(configured_key) < Min_key_length {
			log.WithFields(log.Fields{
				"path": r.URL.Path}).Warn("internal API request rejected - no INTERNAL_API_KEY is configured")
			write_unauthorized(w)
			return
		}

		if !Token_valid(configured_key, Request_token(r)) {
			log.WithFields(log.Fields{
				"path": r.URL.Path}).Warn("internal API request rejected - invalid or missing API key")
			write_unauthorized(w)
			return
		}

		next(w, r)
	}
}

func write_unauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", `Bearer realm="boltcard internal API"`)
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"status":"ERROR","reason":"unauthorized"}`))
}
