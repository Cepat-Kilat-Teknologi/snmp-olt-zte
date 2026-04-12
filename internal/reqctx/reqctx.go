// Package reqctx defines context keys and helpers for per-request values
// that need to flow across package boundaries (logger, middleware, handlers,
// error responses). Keeping these in a leaf package with no internal
// dependencies avoids import cycles between utils, middleware, and logger.
package reqctx

import "context"

// requestIDKeyType is the unexported context key type for the request ID.
// Using a struct type prevents collisions with other context key values.
type requestIDKeyType struct{}

// RequestIDKey is the context key used to store/retrieve the request ID.
var RequestIDKey = requestIDKeyType{}

// WithRequestID returns a new context with the given request ID attached.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, RequestIDKey, id)
}

// RequestIDFromContext extracts the request ID from a context.
// Returns empty string if not present.
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(RequestIDKey).(string); ok {
		return v
	}
	return ""
}
