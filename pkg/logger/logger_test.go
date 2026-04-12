package logger

import (
	"context"
	"sync"
	"testing"

	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/reqctx"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// resetGlobals clears the package-level globals so Init can be exercised
// from a clean slate in multiple tests. Kept in the _test.go file so it
// stays out of production binaries.
func resetGlobals() {
	once = sync.Once{}
	appLogger = nil
}

func newObservedLogger(t *testing.T) (*zap.Logger, *observer.ObservedLogs) {
	t.Helper()
	core, logs := observer.New(zap.DebugLevel)
	return zap.New(core), logs
}

func TestSetForTestRestore(t *testing.T) {
	resetGlobals()
	defer resetGlobals()

	obs, _ := newObservedLogger(t)
	restore := SetForTest(obs)
	require.Same(t, obs, appLogger)
	restore()
	require.Nil(t, appLogger, "restore should revert appLogger to previous value (nil)")
}

func TestLInitializesWhenNil(t *testing.T) {
	resetGlobals()
	defer resetGlobals()

	got := L()
	require.NotNil(t, got)
	require.NotNil(t, appLogger)
	// second call should return the same instance (no re-init)
	require.Same(t, got, L())
}

func TestInitDevelopment(t *testing.T) {
	resetGlobals()
	defer resetGlobals()

	Init("development", "svc", "v1.0.0")
	require.NotNil(t, appLogger)
	// Repeated Init calls are no-ops due to sync.Once
	first := appLogger
	Init("production", "other", "v2.0.0")
	require.Same(t, first, appLogger)
}

func TestInitDevShortAlias(t *testing.T) {
	resetGlobals()
	defer resetGlobals()

	Init("dev", "svc", "v1.0.0")
	require.NotNil(t, appLogger)
}

func TestInitProduction(t *testing.T) {
	resetGlobals()
	defer resetGlobals()

	Init("production", "svc", "v1.0.0")
	require.NotNil(t, appLogger)
}

func TestInitUnknownEnvDefaultsToProduction(t *testing.T) {
	resetGlobals()
	defer resetGlobals()

	Init("staging", "svc", "v1.0.0")
	require.NotNil(t, appLogger)
}

func TestWithRequestIDNoID(t *testing.T) {
	resetGlobals()
	defer resetGlobals()

	obs, logs := newObservedLogger(t)
	restore := SetForTest(obs)
	defer restore()

	lg := WithRequestID(context.Background())
	lg.Info("hello")

	require.Equal(t, 1, logs.Len())
	entry := logs.All()[0]
	require.Equal(t, "hello", entry.Message)
	// no request_id field should be present
	for _, f := range entry.Context {
		require.NotEqual(t, "request_id", f.Key)
	}
}

func TestWithRequestIDWithID(t *testing.T) {
	resetGlobals()
	defer resetGlobals()

	obs, logs := newObservedLogger(t)
	restore := SetForTest(obs)
	defer restore()

	ctx := reqctx.WithRequestID(context.Background(), "abc-123")
	lg := WithRequestID(ctx)
	lg.Info("hello")

	require.Equal(t, 1, logs.Len())
	entry := logs.All()[0]
	ctxMap := entry.ContextMap()
	require.Equal(t, "abc-123", ctxMap["request_id"])
}

func TestWithModule(t *testing.T) {
	resetGlobals()
	defer resetGlobals()

	obs, logs := newObservedLogger(t)
	restore := SetForTest(obs)
	defer restore()

	lg := WithModule("snmp")
	lg.Info("tick")

	require.Equal(t, 1, logs.Len())
	require.Equal(t, "snmp", logs.All()[0].ContextMap()["module"])
}

func TestLevelWrappers(t *testing.T) {
	resetGlobals()
	defer resetGlobals()

	obs, logs := newObservedLogger(t)
	restore := SetForTest(obs)
	defer restore()

	Debug("d", zap.String("k", "v"))
	Info("i")
	Warn("w")
	Error("e")

	require.Equal(t, 4, logs.Len())

	entries := logs.All()
	require.Equal(t, zapcore.DebugLevel, entries[0].Level)
	require.Equal(t, "d", entries[0].Message)
	require.Equal(t, "v", entries[0].ContextMap()["k"])

	require.Equal(t, zapcore.InfoLevel, entries[1].Level)
	require.Equal(t, "i", entries[1].Message)

	require.Equal(t, zapcore.WarnLevel, entries[2].Level)
	require.Equal(t, "w", entries[2].Message)

	require.Equal(t, zapcore.ErrorLevel, entries[3].Level)
	require.Equal(t, "e", entries[3].Message)
}

func TestSyncNilLogger(t *testing.T) {
	resetGlobals()
	defer resetGlobals()

	require.NoError(t, Sync())
}

func TestSyncWithLogger(t *testing.T) {
	resetGlobals()
	defer resetGlobals()

	obs, _ := newObservedLogger(t)
	// Install via SetForTest so Sync() sees a non-nil appLogger
	restore := SetForTest(obs)
	defer restore()

	// Sync on an observer-backed zap.Logger returns nil.
	require.NoError(t, Sync())
}
