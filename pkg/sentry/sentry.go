// Package sentry provides a thin wrapper around the Sentry Go SDK for
// centralized error tracking. When SENTRY_DSN is empty Init is a no-op,
// so the service starts cleanly in development without any Sentry config.
package sentry

import (
	"time"

	sentrygo "github.com/getsentry/sentry-go"
)

// Init initializes the Sentry SDK. A blank dsn is treated as "disabled" and
// returns nil immediately, making the call safe in any environment.
func Init(dsn, env, release string) error {
	if dsn == "" {
		return nil
	}
	return sentrygo.Init(sentrygo.ClientOptions{
		Dsn:              dsn,
		Environment:      env,
		Release:          release,
		TracesSampleRate: 0.1,
		EnableTracing:    true,
	})
}

// CaptureException sends an error event to Sentry.
func CaptureException(err error) {
	sentrygo.CaptureException(err)
}

// CaptureMessage sends a plain-text message event to Sentry.
func CaptureMessage(msg string) {
	sentrygo.CaptureMessage(msg)
}

// Flush waits up to 2 seconds for buffered events to be sent. Call it
// (via defer) before the process exits.
func Flush() {
	sentrygo.Flush(2 * time.Second)
}
