package reqctx

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithRequestID(t *testing.T) {
	ctx := context.Background()
	newCtx := WithRequestID(ctx, "req-123")

	assert.NotEqual(t, ctx, newCtx, "expected a new context instance")
	got, ok := newCtx.Value(RequestIDKey).(string)
	assert.True(t, ok, "expected string value at RequestIDKey")
	assert.Equal(t, "req-123", got)
}

func TestRequestIDFromContext(t *testing.T) {
	t.Run("nil context returns empty", func(t *testing.T) {
		//nolint:staticcheck // intentionally passing nil to exercise the guard
		assert.Equal(t, "", RequestIDFromContext(nil))
	})

	t.Run("context without ID returns empty", func(t *testing.T) {
		assert.Equal(t, "", RequestIDFromContext(context.Background()))
	})

	t.Run("context with ID returns value", func(t *testing.T) {
		ctx := WithRequestID(context.Background(), "abc-xyz")
		assert.Equal(t, "abc-xyz", RequestIDFromContext(ctx))
	})

	t.Run("context with wrong-typed value returns empty", func(t *testing.T) {
		// Value stored under the key is not a string: the type assertion
		// inside RequestIDFromContext should fail and return "".
		ctx := context.WithValue(context.Background(), RequestIDKey, 42)
		assert.Equal(t, "", RequestIDFromContext(ctx))
	})
}
