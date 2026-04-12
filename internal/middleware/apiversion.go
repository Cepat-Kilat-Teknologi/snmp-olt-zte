package middleware

import "net/http"

// APIVersionConfig configures the API version header middleware.
// Use default values by calling DefaultAPIVersionConfig; override individual
// fields if you need non-standard header names.
type APIVersionConfig struct {
	// APIVersion is the contract/schema version exposed to clients (e.g. "v1").
	APIVersion string
	// APIVersionHeader is the response header name for APIVersion.
	// Defaults to "X-API-Version".
	APIVersionHeader string

	// AppVersion is the build/release version of the service binary
	// (e.g. "3.0.0") — typically injected via -ldflags.
	AppVersion string
	// AppVersionHeader is the response header name for AppVersion.
	// Defaults to "X-App-Version".
	AppVersionHeader string

	// Commit is the short git commit hash of the build (optional).
	Commit string
	// CommitHeader is the response header name for Commit.
	// Defaults to "X-Build-Commit".
	CommitHeader string
}

// DefaultAPIVersionConfig returns a config with the ISP-adapter-standard header
// names populated. Pass it to APIVersionHeader to get consistent version
// exposure across all adapters.
func DefaultAPIVersionConfig(apiVersion, appVersion, commit string) APIVersionConfig {
	return APIVersionConfig{
		APIVersion:       apiVersion,
		APIVersionHeader: "X-API-Version",
		AppVersion:       appVersion,
		AppVersionHeader: "X-App-Version",
		Commit:           commit,
		CommitHeader:     "X-Build-Commit",
	}
}

// APIVersionHeader returns a chi middleware that stamps version headers on
// every response. Empty fields are skipped so partial configuration (e.g.
// commit not yet wired into ldflags) degrades gracefully.
//
// Headers added:
//   - X-API-Version  — API contract version, e.g. "v1"
//   - X-App-Version  — binary version, e.g. "3.0.0"
//   - X-Build-Commit — short git SHA
func APIVersionHeader(cfg APIVersionConfig) func(next http.Handler) http.Handler {
	if cfg.APIVersionHeader == "" {
		cfg.APIVersionHeader = "X-API-Version"
	}
	if cfg.AppVersionHeader == "" {
		cfg.AppVersionHeader = "X-App-Version"
	}
	if cfg.CommitHeader == "" {
		cfg.CommitHeader = "X-Build-Commit"
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			if cfg.APIVersion != "" {
				h.Set(cfg.APIVersionHeader, cfg.APIVersion)
			}
			if cfg.AppVersion != "" {
				h.Set(cfg.AppVersionHeader, cfg.AppVersion)
			}
			if cfg.Commit != "" {
				h.Set(cfg.CommitHeader, cfg.Commit)
			}
			next.ServeHTTP(w, r)
		})
	}
}
