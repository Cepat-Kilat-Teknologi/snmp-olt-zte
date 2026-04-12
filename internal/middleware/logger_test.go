package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/pkg/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// newCapturingLogger returns a zap.Logger that writes JSON-encoded log entries
// to the provided buffer. Used by tests that need to inspect middleware output.
func newCapturingLogger(buf *bytes.Buffer) *zap.Logger {
	encCfg := zap.NewProductionEncoderConfig()
	encCfg.TimeKey = "timestamp"
	encCfg.MessageKey = "message"
	encCfg.LevelKey = "level"
	encCfg.CallerKey = ""
	encCfg.StacktraceKey = ""
	enc := zapcore.NewJSONEncoder(encCfg)
	core := zapcore.NewCore(enc, zapcore.AddSync(buf), zapcore.DebugLevel)
	return zap.New(core)
}

// installCapturingLogger swaps the global logger with one that writes to buf,
// returning a restore function to revert after the test.
func installCapturingLogger(t *testing.T, buf *bytes.Buffer) func() {
	t.Helper()
	return logger.SetForTest(newCapturingLogger(buf))
}

// decodeFirstLog parses the first JSON log entry from buf. Tests can then
// assert on individual fields.
func decodeFirstLog(t *testing.T, buf *bytes.Buffer) map[string]interface{} {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(buf.Bytes()))
	var entry map[string]interface{}
	if err := dec.Decode(&entry); err != nil {
		t.Fatalf("failed to parse log as JSON: %v (raw=%q)", err, buf.String())
	}
	return entry
}

func TestLogger_SuccessfulRequest(t *testing.T) {
	var buf bytes.Buffer
	restore := installCapturingLogger(t, &buf)
	defer restore()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	})

	handler := Logger()(testHandler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("User-Agent", "test-agent")
	req.RemoteAddr = "192.168.1.1:12345"

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %d", rr.Code)
	}

	if buf.Len() == 0 {
		t.Fatal("Expected log output, got empty string")
	}

	entry := decodeFirstLog(t, &buf)

	if entry["level"] != "info" {
		t.Errorf("Expected level 'info', got %v", entry["level"])
	}
	if entry["message"] != "incoming_request" {
		t.Errorf("Expected message 'incoming_request', got %v", entry["message"])
	}
	if entry["method"] != "GET" {
		t.Errorf("Expected method 'GET', got %v", entry["method"])
	}
	if entry["path"] != "/api/test" {
		t.Errorf("Expected path '/api/test', got %v", entry["path"])
	}
	if entry["status"] != float64(200) {
		t.Errorf("Expected status 200, got %v", entry["status"])
	}
}

func TestLogger_ErrorResponse(t *testing.T) {
	var buf bytes.Buffer
	restore := installCapturingLogger(t, &buf)
	defer restore()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error"))
	})

	handler := Logger()(testHandler)

	req := httptest.NewRequest("POST", "/api/error", bytes.NewBufferString("test body"))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", rr.Code)
	}

	entry := decodeFirstLog(t, &buf)
	if entry["status"] != float64(500) {
		t.Errorf("Expected status 500, got %v", entry["status"])
	}
	if entry["method"] != "POST" {
		t.Errorf("Expected method 'POST', got %v", entry["method"])
	}
}

func TestLogger_PanicRecovery(t *testing.T) {
	var buf bytes.Buffer
	restore := installCapturingLogger(t, &buf)
	defer restore()

	testHandler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		panic("test panic")
	})

	handler := Logger()(testHandler)

	req := httptest.NewRequest("GET", "/api/panic", nil)
	rr := httptest.NewRecorder()

	// Should not panic - middleware should recover
	handler.ServeHTTP(rr, req)

	if buf.Len() == 0 {
		t.Fatal("Expected log output for panic, got empty string")
	}

	// The panic recovery should log at ERROR level with message "incoming_request_panic"
	if !bytes.Contains(buf.Bytes(), []byte(`"level":"error"`)) {
		t.Error("Expected error level log for panic")
	}
	if !bytes.Contains(buf.Bytes(), []byte("incoming_request_panic")) {
		t.Error("Expected 'incoming_request_panic' message in log")
	}
}

func TestLogger_DifferentMethods(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			var buf bytes.Buffer
			restore := installCapturingLogger(t, &buf)
			defer restore()

			testHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler := Logger()(testHandler)

			req := httptest.NewRequest(method, "/api/test", nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			entry := decodeFirstLog(t, &buf)
			if entry["method"] != method {
				t.Errorf("Expected method '%s', got %v", method, entry["method"])
			}
		})
	}
}

func TestLogger_LogsDurationMs(t *testing.T) {
	var buf bytes.Buffer
	restore := installCapturingLogger(t, &buf)
	defer restore()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := Logger()(testHandler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	entry := decodeFirstLog(t, &buf)
	if _, ok := entry["duration_ms"]; !ok {
		t.Error("Expected 'duration_ms' field in log")
	}
}

func TestLogger_BytesInOut(t *testing.T) {
	var buf bytes.Buffer
	restore := installCapturingLogger(t, &buf)
	defer restore()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("response body"))
	})

	handler := Logger()(testHandler)

	requestBody := "request data"
	req := httptest.NewRequest("POST", "/api/test", bytes.NewBufferString(requestBody))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	entry := decodeFirstLog(t, &buf)
	if _, ok := entry["bytes_in"]; !ok {
		t.Error("Expected 'bytes_in' field in log")
	}
	if _, ok := entry["bytes_out"]; !ok {
		t.Error("Expected 'bytes_out' field in log")
	}
}

func TestLogger_SkipsHealthEndpoints(t *testing.T) {
	skipPaths := []string{"/health", "/healthz", "/ready", "/readyz", "/metrics"}

	for _, path := range skipPaths {
		t.Run(path, func(t *testing.T) {
			var buf bytes.Buffer
			restore := installCapturingLogger(t, &buf)
			defer restore()

			testHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler := Logger()(testHandler)

			req := httptest.NewRequest("GET", path, nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if buf.Len() != 0 {
				t.Errorf("Expected no log output for %s, got: %s", path, buf.String())
			}
		})
	}
}
