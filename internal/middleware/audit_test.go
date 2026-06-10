package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/reqctx"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// installObserver swaps the global logger for a zap observer and returns
// the observed logs plus a cleanup. It MUST be called before AuditLog() is
// invoked, because AuditLog captures logger.L() at construction time.
func installObserver(t *testing.T) (*observer.ObservedLogs, func()) {
	t.Helper()
	core, logs := observer.New(zap.DebugLevel)
	restore := logger.SetForTest(zap.New(core))
	return logs, restore
}

// runMiddleware builds a fresh AuditLog middleware (capturing the currently
// installed logger) and runs the request through a trivial 200 OK handler.
func runMiddleware(t *testing.T, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	mw := AuditLog()
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func TestAuditLog_ReadMethodsAreSkipped(t *testing.T) {
	readMethods := []string{http.MethodGet, http.MethodHead, http.MethodOptions}
	for _, m := range readMethods {
		t.Run(m, func(t *testing.T) {
			logs, restore := installObserver(t)
			defer restore()

			req := httptest.NewRequest(m, "/read", nil)
			runMiddleware(t, req)

			assert.Equal(t, 0, logs.Len(), "read methods should not emit audit logs")
		})
	}
}

func TestAuditLog_WriteMethodsEmitEntry(t *testing.T) {
	writeMethods := []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}
	for _, m := range writeMethods {
		t.Run(m, func(t *testing.T) {
			logs, restore := installObserver(t)
			defer restore()

			req := httptest.NewRequest(m, "/write", nil)
			runMiddleware(t, req)

			entries := logs.All()
			require.Len(t, entries, 1)
			entry := entries[0]
			assert.Equal(t, "audit", entry.Message)
			assert.Equal(t, "audit", entry.LoggerName)

			fields := entry.ContextMap()
			assert.Equal(t, m, fields["method"])
			assert.Equal(t, "/write", fields["path"])
			assert.EqualValues(t, http.StatusOK, fields["status"])
			assert.Equal(t, "(none)", fields["api_key"])
			// httptest.NewRequest leaves ContentLength=0 when body is nil.
			assert.EqualValues(t, 0, fields["body_size"])
			// duration_ms must exist (may legitimately be 0 on a fast path).
			_, ok := fields["duration_ms"]
			assert.True(t, ok, "duration_ms field missing")
		})
	}
}

func TestAuditLog_APIKeyMasking(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{"no key", "", "(none)"},
		{"short key", "abc", "***"},
		{"exactly 8 chars", "abcdefgh", "***"},
		{"long key", "abcdefgh1234", "abcdefgh***"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logs, restore := installObserver(t)
			defer restore()

			req := httptest.NewRequest(http.MethodPost, "/x", nil)
			if tc.header != "" {
				req.Header.Set("X-API-Key", tc.header)
			}
			runMiddleware(t, req)

			require.Equal(t, 1, logs.Len())
			assert.Equal(t, tc.want, logs.All()[0].ContextMap()["api_key"])
		})
	}
}

func TestAuditLog_ClientIPResolution(t *testing.T) {
	tests := []struct {
		name       string
		xff        string
		xRealIP    string
		remoteAddr string
		want       string
	}{
		{
			name:       "x-forwarded-for chain uses first entry trimmed",
			xff:        "203.0.113.1, 10.0.0.2, 10.0.0.3",
			remoteAddr: "192.0.2.1:1234",
			want:       "203.0.113.1",
		},
		{
			name:       "x-forwarded-for single entry",
			xff:        "  203.0.113.5  ",
			remoteAddr: "192.0.2.1:1234",
			want:       "203.0.113.5",
		},
		{
			name:       "x-real-ip when no x-forwarded-for",
			xRealIP:    "198.51.100.7",
			remoteAddr: "192.0.2.1:1234",
			want:       "198.51.100.7",
		},
		{
			name:       "remoteaddr fallback",
			remoteAddr: "192.0.2.1:1234",
			want:       "192.0.2.1:1234",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logs, restore := installObserver(t)
			defer restore()

			req := httptest.NewRequest(http.MethodPost, "/ip", nil)
			if tc.xff != "" {
				req.Header.Set("X-Forwarded-For", tc.xff)
			}
			if tc.xRealIP != "" {
				req.Header.Set("X-Real-IP", tc.xRealIP)
			}
			if tc.remoteAddr != "" {
				req.RemoteAddr = tc.remoteAddr
			}
			runMiddleware(t, req)

			require.Equal(t, 1, logs.Len())
			assert.Equal(t, tc.want, logs.All()[0].ContextMap()["ip"])
		})
	}
}

func TestAuditLog_RequestIDFromContext(t *testing.T) {
	logs, restore := installObserver(t)
	defer restore()

	ctx := reqctx.WithRequestID(context.Background(), "req-xyz")
	req := httptest.NewRequest(http.MethodPost, "/ctx", nil).WithContext(ctx)
	runMiddleware(t, req)

	require.Equal(t, 1, logs.Len())
	assert.Equal(t, "req-xyz", logs.All()[0].ContextMap()["request_id"])
}
