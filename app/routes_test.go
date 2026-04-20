package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/handler"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/health"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/model"
)

// mockOnuUsecase for testing routes
type mockOnuUsecase struct{}

func (m *mockOnuUsecase) GetByBoardIDAndPonID(ctx context.Context, boardID, ponID int) ([]model.ONUInfoPerBoard, error) {
	return nil, nil
}

func (m *mockOnuUsecase) GetByBoardIDPonIDAndOnuID(ctx context.Context, boardID, ponID, onuID int) (model.ONUCustomerInfo, error) {
	return model.ONUCustomerInfo{}, nil
}

func (m *mockOnuUsecase) GetEmptyOnuID(ctx context.Context, boardID, ponID int) ([]model.OnuID, error) {
	return nil, nil
}

func (m *mockOnuUsecase) GetOnuIDAndSerialNumber(ctx context.Context, boardID, ponID int) ([]model.OnuSerialNumber, error) {
	return nil, nil
}

func (m *mockOnuUsecase) UpdateEmptyOnuID(ctx context.Context, boardID, ponID int) error {
	return nil
}

func (m *mockOnuUsecase) GetByBoardIDAndPonIDWithPagination(ctx context.Context, boardID, ponID, page, pageSize int) ([]model.ONUInfoPerBoard, int) {
	return nil, 0
}

func (m *mockOnuUsecase) DeleteCache(ctx context.Context, boardID, ponID int) error {
	return nil
}

func (m *mockOnuUsecase) InvalidateONUCache(_ context.Context, _, _, _ int) error { return nil }
func (m *mockOnuUsecase) PreWarmCache(ctx context.Context)                        {}

func TestRootHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	rootHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %d", rr.Code)
	}

	expectedBody := "Hello, this is the root endpoint!"
	if rr.Body.String() != expectedBody {
		t.Errorf("Expected body '%s', got '%s'", expectedBody, rr.Body.String())
	}
}

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()

	healthHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %d", rr.Code)
	}

	expectedContentType := "application/json"
	if ct := rr.Header().Get("Content-Type"); ct != expectedContentType {
		t.Errorf("Expected Content-Type '%s', got '%s'", expectedContentType, ct)
	}

	// healthHandler now writes JSON via encoding/json which appends a newline.
	expectedBody := "{\"status\":\"healthy\"}\n"
	if rr.Body.String() != expectedBody {
		t.Errorf("Expected body %q, got %q", expectedBody, rr.Body.String())
	}
}

func TestLoadRoutes(t *testing.T) {
	usecase := &mockOnuUsecase{}
	onuHandler := handler.NewOnuHandler(usecase)

	router := loadRoutes(onuHandler, nil)

	if router == nil {
		t.Error("Expected non-nil router")
	}
}

func TestLoadRoutes_RootEndpoint(t *testing.T) {
	usecase := &mockOnuUsecase{}
	onuHandler := handler.NewOnuHandler(usecase)
	router := loadRoutes(onuHandler, nil)

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status OK for root endpoint, got %d", rr.Code)
	}
}

func TestLoadRoutes_MiddlewareApplied(t *testing.T) {
	usecase := &mockOnuUsecase{}
	onuHandler := handler.NewOnuHandler(usecase)
	router := loadRoutes(onuHandler, nil)

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	// Check if RequestID middleware added the header
	requestID := rr.Header().Get("X-Request-ID")
	if requestID == "" {
		t.Error("Expected X-Request-ID header to be set by middleware")
	}

	// Check if security headers are set
	if rr.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("Expected X-Content-Type-Options header to be set")
	}
}

func TestLoadRoutes_CORSHeaders(t *testing.T) {
	usecase := &mockOnuUsecase{}
	onuHandler := handler.NewOnuHandler(usecase)
	router := loadRoutes(onuHandler, nil)

	req := httptest.NewRequest("OPTIONS", "/", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	// CORS middleware should handle OPTIONS requests
	if rr.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Error("Expected CORS headers to be set")
	}
}

func TestHealthzHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/healthz", nil)
	rr := httptest.NewRecorder()

	healthzHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %d", rr.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "healthy" {
		t.Errorf("Expected status=healthy, got %v", body["status"])
	}
}

func TestReadyzHandler_NilChecker(t *testing.T) {
	req := httptest.NewRequest("GET", "/readyz", nil)
	rr := httptest.NewRecorder()

	makeReadyzHandler(nil).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status OK with nil checker, got %d", rr.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ready" {
		t.Errorf("Expected status=ready, got %v", body["status"])
	}
}

func TestReadyzHandler_AllProbesUp(t *testing.T) {
	checker := health.NewChecker(time.Second)
	checker.Register("redis", time.Second, func(_ context.Context) error { return nil })
	checker.Register("snmp", time.Second, func(_ context.Context) error { return nil })

	req := httptest.NewRequest("GET", "/readyz", nil)
	rr := httptest.NewRecorder()

	makeReadyzHandler(checker).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 when all probes up, got %d", rr.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ready" {
		t.Errorf("Expected status=ready, got %v", body["status"])
	}
	deps, ok := body["dependencies"].(map[string]any)
	if !ok {
		t.Fatalf("Expected dependencies map, got %T", body["dependencies"])
	}
	if len(deps) != 2 {
		t.Errorf("Expected 2 dependencies, got %d", len(deps))
	}
	for name, entry := range deps {
		m, ok := entry.(map[string]any)
		if !ok {
			t.Errorf("dep %s: expected map, got %T", name, entry)
			continue
		}
		if m["state"] != "up" {
			t.Errorf("dep %s: expected state=up, got %v", name, m["state"])
		}
	}
}

func TestReadyzHandler_OneProbeDown(t *testing.T) {
	checker := health.NewChecker(time.Second)
	checker.Register("redis", time.Second, func(_ context.Context) error { return nil })
	checker.Register("snmp", time.Second, func(_ context.Context) error {
		return errors.New("olt unreachable")
	})

	req := httptest.NewRequest("GET", "/readyz", nil)
	rr := httptest.NewRecorder()

	makeReadyzHandler(checker).ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected 503 when probe down, got %d", rr.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "not_ready" {
		t.Errorf("Expected status=not_ready, got %v", body["status"])
	}
	deps := body["dependencies"].(map[string]any)
	snmp := deps["snmp"].(map[string]any)
	if snmp["state"] != "down" {
		t.Errorf("Expected snmp state=down, got %v", snmp["state"])
	}
	if snmp["error"] != "olt unreachable" {
		t.Errorf("Expected snmp error message, got %v", snmp["error"])
	}
}

func TestLoadRoutes_MetricsEndpoint(t *testing.T) {
	usecase := &mockOnuUsecase{}
	onuHandler := handler.NewOnuHandler(usecase)
	router := loadRoutes(onuHandler, nil)

	// Issue a normal request first so the metrics middleware records
	// at least one sample. Otherwise counter vectors without any label
	// observations are omitted from the Prometheus text exposition.
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/nonexistent", nil))

	req := httptest.NewRequest("GET", "/metrics", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 from /metrics, got %d", rr.Code)
	}
	if !bytesContains(rr.Body.Bytes(), "http_requests_total") {
		t.Error("Expected /metrics body to contain http_requests_total")
	}
}

func TestLoadRoutes_HealthzEndpoint(t *testing.T) {
	usecase := &mockOnuUsecase{}
	onuHandler := handler.NewOnuHandler(usecase)
	router := loadRoutes(onuHandler, nil)

	req := httptest.NewRequest("GET", "/healthz", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 from /healthz, got %d", rr.Code)
	}
}

func TestLoadRoutes_ReadyzEndpoint(t *testing.T) {
	usecase := &mockOnuUsecase{}
	onuHandler := handler.NewOnuHandler(usecase)
	router := loadRoutes(onuHandler, nil)

	req := httptest.NewRequest("GET", "/readyz", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 from /readyz, got %d", rr.Code)
	}
}

func TestVersionHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/version", nil)
	rr := httptest.NewRecorder()
	versionHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 from /version, got %d", rr.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, key := range []string{"version", "api_version", "commit", "build_time", "uptime"} {
		if _, ok := body[key]; !ok {
			t.Errorf("missing key %q in /version response: %v", key, body)
		}
	}
}

func TestLoadRoutes_VersionHeadersPresent(t *testing.T) {
	usecase := &mockOnuUsecase{}
	onuHandler := handler.NewOnuHandler(usecase)
	router := loadRoutes(onuHandler, nil)

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	// buildinfo defaults non-empty, so APIVersionHeader must stamp all three.
	if rr.Header().Get("X-API-Version") == "" {
		t.Error("Expected X-API-Version header")
	}
	if rr.Header().Get("X-App-Version") == "" {
		t.Error("Expected X-App-Version header")
	}
	if rr.Header().Get("X-Build-Commit") == "" {
		t.Error("Expected X-Build-Commit header")
	}
}

// bytesContains is a tiny helper so we don't need to import the strings
// package just for a single Contains call in the metrics test.
func bytesContains(haystack []byte, needle string) bool {
	n := len(needle)
	for i := 0; i+n <= len(haystack); i++ {
		if string(haystack[i:i+n]) == needle {
			return true
		}
	}
	return false
}
