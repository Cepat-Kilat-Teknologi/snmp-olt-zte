package middleware

import (
	"net/http"
	"runtime/debug"
	"time"

	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/utils"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/pkg/logger"
	chimw "github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

// skipLogPaths contains URIs that should NOT emit request logs.
// Health and metrics endpoints are excluded to keep logs quiet.
var skipLogPaths = map[string]struct{}{
	"/health":  {},
	"/healthz": {},
	"/ready":   {},
	"/readyz":  {},
	"/metrics": {},
}

// Logger is a middleware that logs incoming HTTP requests using the global
// zap logger. It captures method, path, status, duration, client IP, user
// agent, and request ID, and recovers from panics so they are logged as errors.
func Logger() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			if _, skip := skipLogPaths[r.URL.Path]; skip {
				next.ServeHTTP(w, r)
				return
			}

			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
			startTime := time.Now()

			defer func() {
				elapsed := time.Since(startTime)

				reqID := utils.RequestIDFromContext(r.Context())

				if rec := recover(); rec != nil && rec != http.ErrAbortHandler {
					logger.L().Error("incoming_request_panic",
						zap.Any("recover", rec),
						zap.ByteString("stack", debug.Stack()),
						zap.String("request_id", reqID),
						zap.String("method", r.Method),
						zap.String("path", r.URL.Path),
					)
					ww.WriteHeader(http.StatusInternalServerError)
				}

				logger.L().Info("incoming_request",
					zap.String("request_id", reqID),
					zap.String("remote_addr", r.RemoteAddr),
					zap.String("path", r.URL.Path),
					zap.String("proto", r.Proto),
					zap.String("method", r.Method),
					zap.String("user_agent", r.UserAgent()),
					zap.Int("status", ww.Status()),
					zap.Int64("bytes_in", r.ContentLength),
					zap.Int("bytes_out", ww.BytesWritten()),
					zap.Int64("duration_ms", elapsed.Milliseconds()),
				)
			}()

			next.ServeHTTP(ww, r)
		}

		return http.HandlerFunc(fn)
	}
}
