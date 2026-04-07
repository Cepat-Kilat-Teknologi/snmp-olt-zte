package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/handler"
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

func (m *mockOnuUsecase) PreWarmCache(ctx context.Context) {}

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

	expectedBody := `{"status":"ok"}`
	if rr.Body.String() != expectedBody {
		t.Errorf("Expected body '%s', got '%s'", expectedBody, rr.Body.String())
	}
}

func TestLoadRoutes(t *testing.T) {
	usecase := &mockOnuUsecase{}
	onuHandler := handler.NewOnuHandler(usecase)

	router := loadRoutes(onuHandler)

	if router == nil {
		t.Error("Expected non-nil router")
	}
}

func TestLoadRoutes_RootEndpoint(t *testing.T) {
	usecase := &mockOnuUsecase{}
	onuHandler := handler.NewOnuHandler(usecase)
	router := loadRoutes(onuHandler)

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
	router := loadRoutes(onuHandler)

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
	router := loadRoutes(onuHandler)

	req := httptest.NewRequest("OPTIONS", "/", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	// CORS middleware should handle OPTIONS requests
	if rr.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Error("Expected CORS headers to be set")
	}
}
