package utils

import (
	"context"

	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/reqctx"
)

// RequestIDKey is re-exported from the reqctx package for callers that have
// historically used utils.RequestIDKey. New code should prefer reqctx directly.
var RequestIDKey = reqctx.RequestIDKey

// RequestIDFromContext extracts the request ID from a context.
// Returns empty string if not present.
func RequestIDFromContext(ctx context.Context) string {
	return reqctx.RequestIDFromContext(ctx)
}
