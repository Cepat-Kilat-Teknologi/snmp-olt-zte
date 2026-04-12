// Package buildinfo exposes build-time metadata (version, commit, build
// time) set via -ldflags at compile time. Keeping it in a dedicated package
// avoids cluttering cmd/api/main.go and lets any package in the project read
// the current version for logs, metrics labels, and response headers.
package buildinfo

import "time"

// Values populated via -ldflags -X at build time. Defaults are safe for
// `go run` / `go test` where ldflags are not passed.
var (
	// Version is the semantic version of the binary, e.g. "3.0.0".
	Version = "dev"
	// Commit is the short git SHA at build time.
	Commit = "none"
	// BuildTime is the RFC3339 timestamp of the build (optional).
	BuildTime = "unknown"
	// APIVersion is the contract/schema version exposed on /api/v1.
	// Kept in code rather than ldflags because it is the HTTP contract
	// version, not the build version.
	APIVersion = "v1"
)

// Startup returns the process start time. It is cached on first access so
// uptime can be computed consistently.
var startAt = time.Now()

// Uptime returns the duration since the process started.
func Uptime() time.Duration {
	return time.Since(startAt)
}

// Info returns a compact map with build metadata suitable for serving from
// a /version endpoint or attaching to structured log records.
func Info() map[string]any {
	return map[string]any{
		"version":     Version,
		"api_version": APIVersion,
		"commit":      Commit,
		"build_time":  BuildTime,
		"uptime":      Uptime().String(),
	}
}
