package reqctx

import "context"

// Principal is the authenticated caller resolved from the X-API-Key header in
// multi-tenant mode (the API_USERS registry). It lives in this leaf package so
// middleware can set it and downstream ownership checks can read it without an
// import cycle. When per-user auth is disabled there is no Principal in context.
type Principal struct {
	UserID int  // owner id this caller is scoped to
	Admin  bool // true for the "admin" role — sees every OLT regardless of user_id
}

// principalKeyType is the unexported context key type for the Principal.
type principalKeyType struct{}

// PrincipalKey is the context key used to store/retrieve the Principal.
var PrincipalKey = principalKeyType{}

// WithPrincipal returns a new context with the given Principal attached.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, PrincipalKey, p)
}

// PrincipalFromContext returns the Principal and true when one is present.
// A false second return means per-user auth is not active for this request
// (legacy single-key or open mode) — callers should not enforce ownership.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	if ctx == nil {
		return Principal{}, false
	}
	p, ok := ctx.Value(PrincipalKey).(Principal)
	return p, ok
}
