package reqctx

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithPrincipal(t *testing.T) {
	ctx := WithPrincipal(context.Background(), Principal{UserID: 7, Admin: true})
	got, ok := PrincipalFromContext(ctx)
	assert.True(t, ok)
	assert.Equal(t, 7, got.UserID)
	assert.True(t, got.Admin)
}

func TestPrincipalFromContext(t *testing.T) {
	t.Run("nil context", func(t *testing.T) {
		//nolint:staticcheck // intentionally passing nil to exercise the guard
		_, ok := PrincipalFromContext(nil)
		assert.False(t, ok)
	})

	t.Run("absent principal", func(t *testing.T) {
		_, ok := PrincipalFromContext(context.Background())
		assert.False(t, ok)
	})

	t.Run("wrong-typed value", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), PrincipalKey, "nope")
		_, ok := PrincipalFromContext(ctx)
		assert.False(t, ok)
	})
}
