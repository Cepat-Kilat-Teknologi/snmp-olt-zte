package health

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrNoProbes(t *testing.T) {
	require.NotNil(t, ErrNoProbes)
	assert.Equal(t, "no dependency probes registered", ErrNoProbes.Error())
}

func TestNewChecker(t *testing.T) {
	c := NewChecker(2 * time.Second)
	require.NotNil(t, c)
	assert.Equal(t, 2*time.Second, c.timeout)
	assert.Empty(t, c.deps)
}

func TestChecker_NoProbes(t *testing.T) {
	c := NewChecker(time.Second)
	statuses, healthy := c.Check(context.Background())
	assert.Empty(t, statuses)
	assert.True(t, healthy, "zero probes should report healthy=true")
}

func TestChecker_SingleProbeSuccess(t *testing.T) {
	c := NewChecker(time.Second)
	c.Register("redis", time.Minute, func(ctx context.Context) error {
		return nil
	})

	statuses, healthy := c.Check(context.Background())
	require.Len(t, statuses, 1)
	assert.Equal(t, "redis", statuses[0].Name)
	assert.Equal(t, "up", statuses[0].State)
	assert.Empty(t, statuses[0].Err)
	assert.True(t, healthy)
}

func TestChecker_SingleProbeFailure(t *testing.T) {
	c := NewChecker(time.Second)
	boom := errors.New("connection refused")
	c.Register("redis", time.Minute, func(ctx context.Context) error {
		return boom
	})

	statuses, healthy := c.Check(context.Background())
	require.Len(t, statuses, 1)
	assert.Equal(t, "redis", statuses[0].Name)
	assert.Equal(t, "down", statuses[0].State)
	assert.Equal(t, "connection refused", statuses[0].Err)
	assert.False(t, healthy)
}

func TestChecker_MultipleProbesOneFails(t *testing.T) {
	c := NewChecker(time.Second)
	c.Register("redis", time.Minute, func(ctx context.Context) error {
		return nil
	})
	c.Register("snmp", time.Minute, func(ctx context.Context) error {
		return errors.New("timeout")
	})
	c.Register("mysql", time.Minute, func(ctx context.Context) error {
		return nil
	})

	statuses, healthy := c.Check(context.Background())
	require.Len(t, statuses, 3)
	assert.False(t, healthy)

	byName := map[string]Status{}
	for _, s := range statuses {
		byName[s.Name] = s
	}
	assert.Equal(t, "up", byName["redis"].State)
	assert.Equal(t, "down", byName["snmp"].State)
	assert.Equal(t, "timeout", byName["snmp"].Err)
	assert.Equal(t, "up", byName["mysql"].State)
}

func TestChecker_SuccessIsCachedWithinTTL(t *testing.T) {
	c := NewChecker(time.Second)
	var count int
	c.Register("redis", 50*time.Millisecond, func(ctx context.Context) error {
		count++
		return nil
	})

	_, healthy := c.Check(context.Background())
	assert.True(t, healthy)
	assert.Equal(t, 1, count)

	// Second call within TTL — should be served from cache.
	_, healthy = c.Check(context.Background())
	assert.True(t, healthy)
	assert.Equal(t, 1, count, "probe should not re-run within TTL")
}

func TestChecker_FailureIsCachedBriefly(t *testing.T) {
	c := NewChecker(time.Second)
	var count int
	failing := true
	c.Register("redis", time.Hour, func(ctx context.Context) error {
		count++
		if failing {
			return errors.New("nope")
		}
		return nil
	})

	_, healthy := c.Check(context.Background())
	assert.False(t, healthy)
	assert.Equal(t, 1, count)

	// Within failureTTL the cached failure is served — no re-probe storm.
	_, healthy = c.Check(context.Background())
	assert.False(t, healthy)
	assert.Equal(t, 1, count, "failure must be served from cache within failureTTL")

	// Force the failure cache to expire, then recover: the next call probes
	// again and sees the dependency back up.
	failing = false
	c.deps[0].lastAt = time.Now().Add(-failureTTL - time.Second)
	_, healthy = c.Check(context.Background())
	assert.True(t, healthy)
	assert.Equal(t, 2, count, "expired failure cache must re-probe")
}

func TestChecker_CacheExpiresAfterTTL(t *testing.T) {
	c := NewChecker(time.Second)
	var count int
	c.Register("redis", 20*time.Millisecond, func(ctx context.Context) error {
		count++
		return nil
	})

	_, _ = c.Check(context.Background())
	require.Equal(t, 1, count)

	// Still cached.
	_, _ = c.Check(context.Background())
	require.Equal(t, 1, count)

	// Wait for TTL to expire.
	time.Sleep(40 * time.Millisecond)

	_, healthy := c.Check(context.Background())
	assert.True(t, healthy)
	assert.Equal(t, 2, count, "cached result should be re-probed after TTL")
}

func TestChecker_RespectsTimeout(t *testing.T) {
	c := NewChecker(20 * time.Millisecond)
	c.Register("slow", time.Minute, func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})

	start := time.Now()
	statuses, healthy := c.Check(context.Background())
	elapsed := time.Since(start)

	require.Len(t, statuses, 1)
	assert.False(t, healthy)
	assert.Equal(t, "down", statuses[0].State)
	assert.Equal(t, context.DeadlineExceeded.Error(), statuses[0].Err)
	assert.Less(t, elapsed, 500*time.Millisecond, "should return promptly after timeout fires")
}
