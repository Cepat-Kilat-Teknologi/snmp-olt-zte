package sentry

import (
	"strings"
)

// shortSHALen is the length of an abbreviated git commit SHA used in release
// names for untagged (main) builds.
const shortSHALen = 7

// Resolve returns the Sentry environment and release for this process.
//
// Environment: SENTRY_ENVIRONMENT when non-empty, otherwise fallbackEnv (the
// service's APP_ENV value).
//
// Release: SENTRY_RELEASE when non-empty, otherwise "<service>@<version>".
// version is the git tag on tag builds and the short commit SHA on main
// builds, injected by CI through -ldflags. A full 40-character SHA is
// shortened to 7 characters, and an empty version becomes "dev".
//
// getenv is injected so the lookup order can be tested without touching the
// process environment; production callers pass os.Getenv.
func Resolve(getenv func(string) string, service, fallbackEnv, version string) (environment, release string) {
	environment = strings.TrimSpace(getenv("SENTRY_ENVIRONMENT"))
	if environment == "" {
		environment = fallbackEnv
	}

	release = strings.TrimSpace(getenv("SENTRY_RELEASE"))
	if release == "" {
		release = service + "@" + normalizeVersion(version)
	}
	return environment, release
}

// normalizeVersion maps the ldflags version to the release suffix.
func normalizeVersion(version string) string {
	v := strings.TrimSpace(version)
	if v == "" {
		return "dev"
	}
	if isFullSHA(v) {
		return v[:shortSHALen]
	}
	return v
}

// isFullSHA reports whether s is a 40-character lowercase hex git SHA.
func isFullSHA(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
