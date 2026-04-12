package metrics

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"single id", "/board/1", "/board/:id"},
		{"board pon", "/board/1/pon/8", "/board/:id/pon/:id"},
		{"board pon onu", "/api/v1/board/2/pon/16/onu/128", "/api/v1/board/:id/pon/:id/onu/:id"},
		{"health no ids", "/health", "/health"},
		{"no ids", "/no/ids/here", "/no/ids/here"},
		{"root", "/", "/"},
		{"empty", "", ""},
		{"multi-digit id", "/board/12345", "/board/:id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, NormalizePath(tt.input))
		})
	}
}

func TestMiddleware_RecordsMetrics(t *testing.T) {
	mw := Middleware()
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	}))

	before := testutil.ToFloat64(HTTPRequestsTotal.WithLabelValues("GET", "/board/:id/pon/:id", "201"))

	req := httptest.NewRequest(http.MethodGet, "/board/1/pon/8", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	after := testutil.ToFloat64(HTTPRequestsTotal.WithLabelValues("GET", "/board/:id/pon/:id", "201"))
	assert.Equal(t, before+1, after, "HTTPRequestsTotal should increment by 1")

	// Histogram should have recorded at least one sample.
	count := testutil.CollectAndCount(HTTPRequestDuration)
	assert.Greater(t, count, 0)
}

func TestMiddleware_DefaultStatusWhenHandlerDoesNotWriteHeader(t *testing.T) {
	mw := Middleware()
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No explicit WriteHeader / Write → chi WrapResponseWriter defaults to 200
	}))

	// chi WrapResponseWriter reports Status()==0 when WriteHeader was never called.
	before := testutil.ToFloat64(HTTPRequestsTotal.WithLabelValues("POST", "/api/v1/thing", "0"))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/thing", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	after := testutil.ToFloat64(HTTPRequestsTotal.WithLabelValues("POST", "/api/v1/thing", "0"))
	assert.Equal(t, before+1, after)
}

func TestMiddleware_SkipPaths(t *testing.T) {
	skipped := []string{"/health", "/healthz", "/ready", "/readyz", "/metrics", "/"}

	mw := Middleware()
	called := 0
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(http.StatusOK)
	}))

	// Snapshot total count across all label combinations via CollectAndCount of counter.
	// Because labels are variable, easier: record that no new sample is added for a unique label.
	for _, p := range skipped {
		before := testutil.ToFloat64(HTTPRequestsTotal.WithLabelValues("GET", p, "200"))
		req := httptest.NewRequest(http.MethodGet, p, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		after := testutil.ToFloat64(HTTPRequestsTotal.WithLabelValues("GET", p, "200"))
		assert.Equal(t, before, after, "path %s should be skipped", p)
	}
	assert.Equal(t, len(skipped), called, "inner handler should still be invoked for skipped paths")
}

func TestHandler_ServesMetrics(t *testing.T) {
	// Ensure all metric families have at least one series so they appear in the output.
	RecordCacheHit("test-served")
	RecordCacheMiss("test-served")
	var noErr error
	RecordSNMPOperation("get", time.Now(), &noErr)

	h := Handler()
	require.NotNil(t, h)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	out := string(body)

	// Expect some well-known metric names to appear.
	for _, name := range []string{
		"http_requests_total",
		"snmp_operations_total",
		"snmp_cache_hits_total",
		"snmp_cache_misses_total",
	} {
		assert.True(t, strings.Contains(out, name), "metrics output should contain %s", name)
	}
}

func TestRecordSNMPOperation_Success(t *testing.T) {
	before := testutil.ToFloat64(SNMPOperationsTotal.WithLabelValues("get", "success"))

	var err error
	func() {
		defer RecordSNMPOperation("get", time.Now(), &err)
		// no error
	}()

	after := testutil.ToFloat64(SNMPOperationsTotal.WithLabelValues("get", "success"))
	assert.Equal(t, before+1, after)

	assert.Greater(t, testutil.CollectAndCount(SNMPOperationDuration), 0)
}

func TestRecordSNMPOperation_Error(t *testing.T) {
	before := testutil.ToFloat64(SNMPOperationsTotal.WithLabelValues("walk", "error"))

	err := errors.New("boom")
	RecordSNMPOperation("walk", time.Now().Add(-10*time.Millisecond), &err)

	after := testutil.ToFloat64(SNMPOperationsTotal.WithLabelValues("walk", "error"))
	assert.Equal(t, before+1, after)
}

func TestRecordSNMPOperation_NilErrp(t *testing.T) {
	before := testutil.ToFloat64(SNMPOperationsTotal.WithLabelValues("ping", "success"))

	RecordSNMPOperation("ping", time.Now(), nil)

	after := testutil.ToFloat64(SNMPOperationsTotal.WithLabelValues("ping", "success"))
	assert.Equal(t, before+1, after)
}

func TestRecordCacheHit(t *testing.T) {
	before := testutil.ToFloat64(CacheHitsTotal.WithLabelValues("onu-info"))
	RecordCacheHit("onu-info")
	after := testutil.ToFloat64(CacheHitsTotal.WithLabelValues("onu-info"))
	assert.Equal(t, before+1, after)
}

func TestRecordCacheMiss(t *testing.T) {
	before := testutil.ToFloat64(CacheMissesTotal.WithLabelValues("onu-info"))
	RecordCacheMiss("onu-info")
	after := testutil.ToFloat64(CacheMissesTotal.WithLabelValues("onu-info"))
	assert.Equal(t, before+1, after)
}
