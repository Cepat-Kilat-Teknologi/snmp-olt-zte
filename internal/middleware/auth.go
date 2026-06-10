package middleware

import (
	"crypto/subtle"
	"net/http"
	"os"

	apperrors "github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/errors"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/reqctx"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/utils"
	"github.com/go-chi/chi/v5"
)

// APIKeyAuth validates the X-API-Key header against the API_KEY environment variable.
// If API_KEY is not set, authentication is disabled.
//
// Deprecated: prefer Authenticator, which supports the per-tenant API_USERS
// registry. Kept for backwards compatibility (single-key deployments and tests).
func APIKeyAuth(next http.Handler) http.Handler {
	apiKey := os.Getenv("API_KEY")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if apiKey == "" {
			next.ServeHTTP(w, r)
			return
		}

		key := r.Header.Get("X-API-Key")
		if key != apiKey {
			writeUnauthorized(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Authenticator authenticates the X-API-Key header.
//
//   - When users is non-nil (API_USERS configured) per-tenant auth is active:
//     the key MUST resolve to a registered principal, otherwise 401. The
//     resolved Principal is stored in the request context for downstream
//     ownership checks (RequireOLTOwner).
//   - When users is nil it falls back to the legacy single key: a non-empty
//     legacyKey must match (else 401); an empty legacyKey disables auth.
//
// users maps api_key -> Principal{UserID, Admin}.
func Authenticator(users map[string]reqctx.Principal, legacyKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("X-API-Key")

			if users != nil {
				p, ok := users[key]
				if key == "" || !ok {
					writeUnauthorized(w, r)
					return
				}
				next.ServeHTTP(w, r.WithContext(reqctx.WithPrincipal(r.Context(), p)))
				return
			}

			// Legacy single-key mode (no per-tenant scoping). Constant-time
			// compare so the key can't be recovered via response timing.
			if legacyKey != "" && subtle.ConstantTimeCompare([]byte(key), []byte(legacyKey)) != 1 {
				writeUnauthorized(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireOLTOwner authorizes access to an OLT scoped to oltUserID. It allows the
// request when there is no Principal (per-user auth disabled), when the caller
// is an admin, or when the caller's user_id matches the OLT's owner. Otherwise
// it responds 404 — NOT 403 — so a tenant cannot probe for the existence of
// another tenant's OLTs (no enumeration via 403-vs-404).
func RequireOLTOwner(oltUserID int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := reqctx.PrincipalFromContext(r.Context())
			if !ok || p.Admin || p.UserID == oltUserID {
				next.ServeHTTP(w, r)
				return
			}
			// Echo only the id the caller asked for (empty on bare /board paths).
			utils.ErrorNotFound(w, r, apperrors.NewNotFoundError("olt", chi.URLParam(r, "olt_id")))
		})
	}
}

// writeUnauthorized emits the standard error envelope (code/status/error_code/
// data/request_id) for auth failures, consistent with every other error path.
func writeUnauthorized(w http.ResponseWriter, r *http.Request) {
	utils.ErrorUnauthorized(w, r, apperrors.NewUnauthorizedError("invalid or missing API key"))
}
