package middleware

import (
	"context"
	"net/http"

	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/utils"
	"github.com/rs/xid"
)

// RequestIDKey is re-exported from utils for backwards compatibility with tests
// and consumers that reference middleware.RequestIDKey.
var RequestIDKey = utils.RequestIDKey

// RequestID adds a unique request ID to each request for tracking and debugging.
// The request ID can be provided by the client via X-Request-ID header,
// or will be generated automatically if not provided.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to get request ID from header first (from client or load balancer)
		requestID := r.Header.Get("X-Request-ID")

		// If not provided, generate a new unique ID
		if requestID == "" {
			requestID = xid.New().String()
		}

		// Add request ID to the response header for client tracking
		w.Header().Set("X-Request-ID", requestID)

		// Add request ID to request context for use in handlers and logging
		ctx := context.WithValue(r.Context(), utils.RequestIDKey, requestID)

		// Continue with the request
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
