// Package safego contains a recover helper for goroutines.
//
// The payment and notification paths are run in their own goroutines. A panic
// there is not recovered by the HTTP server, so without this it would stop the
// whole card service - an unreachable lightning node or an unexpected response
// from another service must not take payments offline.
package safego

import (
	log "github.com/sirupsen/logrus"
)

// Recover logs a panic raised in a goroutine instead of letting it stop the
// service. Call it as the first statement, as `defer safego.Recover("name")`.
func Recover(name string) {
	if r := recover(); r != nil {
		log.WithFields(log.Fields{"goroutine": name}).Error("recovered from panic: ", r)
	}
}
