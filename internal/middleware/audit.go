package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/reqctx"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/pkg/logger"
	chimw "github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

// AuditLog returns a middleware that emits a separate structured audit log
// entry for every write operation (POST/PUT/PATCH/DELETE). Read operations
// (GET/HEAD/OPTIONS) are skipped to keep the audit channel focused on changes.
//
// The audit entry is written via the named sub-logger "audit" so log
// aggregators can route it to compliance storage separately from the
// normal request log stream. Fields intentionally match the schema shared
// across all ISP adapters (see architecture-isp-app/docs/LOGGING.md §7).
func AuditLog() func(next http.Handler) http.Handler {
	auditLogger := logger.L().Named("audit")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isReadMethod(r.Method) {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r)

			auditLogger.Info("audit",
				zap.String("request_id", reqctx.RequestIDFromContext(r.Context())),
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", ww.Status()),
				zap.String("ip", clientIP(r)),
				zap.String("api_key", maskAPIKey(r.Header.Get("X-API-Key"))),
				zap.String("user_agent", r.UserAgent()),
				zap.Int64("body_size", r.ContentLength),
				zap.Int64("duration_ms", time.Since(start).Milliseconds()),
			)
		})
	}
}

// isReadMethod reports whether the HTTP method is a non-mutating (read-only)
// method that should be excluded from the audit log.
func isReadMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}

// maskAPIKey returns a redacted form of the API key suitable for audit logs.
// The first 8 characters are preserved so operators can correlate entries
// with key provisioning records without exposing the full secret.
func maskAPIKey(key string) string {
	if key == "" {
		return "(none)"
	}
	if len(key) <= 8 {
		return "***"
	}
	return key[:8] + "***"
}

// clientIP returns the best-effort client IP for the request. It honors
// X-Forwarded-For and X-Real-IP headers if present (common behind load
// balancers/reverse proxies) and falls back to RemoteAddr otherwise.
func clientIP(r *http.Request) string {
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		// X-Forwarded-For may contain a comma-separated chain; the leftmost
		// entry is the original client.
		if idx := strings.IndexByte(v, ','); idx >= 0 {
			return strings.TrimSpace(v[:idx])
		}
		return strings.TrimSpace(v)
	}
	if v := r.Header.Get("X-Real-IP"); v != "" {
		return strings.TrimSpace(v)
	}
	return r.RemoteAddr
}
