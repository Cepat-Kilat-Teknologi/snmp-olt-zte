package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/middleware"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/model"
	"github.com/go-chi/chi/v5"
)

// mockOnuUsecase is a mock implementation of OnuUseCaseInterface
type mockOnuUsecase struct {
	GetByBoardIDAndPonIDFunc               func(ctx context.Context, boardID, ponID int) ([]model.ONUInfoPerBoard, error)
	GetByBoardIDPonIDAndOnuIDFunc          func(ctx context.Context, boardID, ponID, onuID int) (model.ONUCustomerInfo, error)
	GetEmptyOnuIDFunc                      func(ctx context.Context, boardID, ponID int) ([]model.OnuID, error)
	GetOnuIDAndSerialNumberFunc            func(ctx context.Context, boardID, ponID int) ([]model.OnuSerialNumber, error)
	UpdateEmptyOnuIDFunc                   func(ctx context.Context, boardID, ponID int) error
	GetByBoardIDAndPonIDWithPaginationFunc func(ctx context.Context, boardID, ponID, page, pageSize int) ([]model.ONUInfoPerBoard, int)
	DeleteCacheFunc                        func(ctx context.Context, boardID, ponID int) error
}

func (m *mockOnuUsecase) GetByBoardIDAndPonID(ctx context.Context, boardID, ponID int) ([]model.ONUInfoPerBoard, error) {
	if m.GetByBoardIDAndPonIDFunc != nil {
		return m.GetByBoardIDAndPonIDFunc(ctx, boardID, ponID)
	}
	return nil, nil
}

func (m *mockOnuUsecase) GetByBoardIDPonIDAndOnuID(ctx context.Context, boardID, ponID, onuID int) (model.ONUCustomerInfo, error) {
	if m.GetByBoardIDPonIDAndOnuIDFunc != nil {
		return m.GetByBoardIDPonIDAndOnuIDFunc(ctx, boardID, ponID, onuID)
	}
	return model.ONUCustomerInfo{}, nil
}

func (m *mockOnuUsecase) GetEmptyOnuID(ctx context.Context, boardID, ponID int) ([]model.OnuID, error) {
	if m.GetEmptyOnuIDFunc != nil {
		return m.GetEmptyOnuIDFunc(ctx, boardID, ponID)
	}
	return nil, nil
}

func (m *mockOnuUsecase) GetOnuIDAndSerialNumber(ctx context.Context, boardID, ponID int) ([]model.OnuSerialNumber, error) {
	if m.GetOnuIDAndSerialNumberFunc != nil {
		return m.GetOnuIDAndSerialNumberFunc(ctx, boardID, ponID)
	}
	return nil, nil
}

func (m *mockOnuUsecase) UpdateEmptyOnuID(ctx context.Context, boardID, ponID int) error {
	if m.UpdateEmptyOnuIDFunc != nil {
		return m.UpdateEmptyOnuIDFunc(ctx, boardID, ponID)
	}
	return nil
}

func (m *mockOnuUsecase) GetByBoardIDAndPonIDWithPagination(ctx context.Context, boardID, ponID, page, pageSize int) ([]model.ONUInfoPerBoard, int) {
	if m.GetByBoardIDAndPonIDWithPaginationFunc != nil {
		return m.GetByBoardIDAndPonIDWithPaginationFunc(ctx, boardID, ponID, page, pageSize)
	}
	return nil, 0
}

func (m *mockOnuUsecase) DeleteCache(ctx context.Context, boardID, ponID int) error {
	if m.DeleteCacheFunc != nil {
		return m.DeleteCacheFunc(ctx, boardID, ponID)
	}
	return nil
}

func (m *mockOnuUsecase) InvalidateONUCache(_ context.Context, _, _, _ int) error { return nil }
func (m *mockOnuUsecase) PreWarmCache(ctx context.Context)                        {}

// TestGetRequestID removed: the local getRequestID helper was replaced by
// logger.WithRequestID(ctx) which uses the context-based reqctx package.
// Request ID propagation is covered by TestHandleError_RequestIDPropagated
// in internal/utils and by the middleware.RequestID tests.

func TestNewOnuHandler(t *testing.T) {
	usecase := &mockOnuUsecase{}
	handler := NewOnuHandler(usecase)

	if handler == nil {
		t.Error("Expected non-nil handler")
	}

	// Verify it implements the interface
	var _ OnuHandlerInterface = handler
}

func TestNewOnuHandler_InitializesUsecase(t *testing.T) {
	usecase := &mockOnuUsecase{}
	handler := NewOnuHandler(usecase)

	if handler.ponUsecase == nil {
		t.Error("Expected ponUsecase to be initialized")
	}

	if handler.ponUsecase != usecase {
		t.Error("Expected ponUsecase to be the same as provided usecase")
	}
}

func TestOnuHandler_GetByBoardIDAndPonID_WithContext(t *testing.T) {
	usecase := &mockOnuUsecase{
		GetByBoardIDAndPonIDFunc: func(ctx context.Context, boardID, ponID int) ([]model.ONUInfoPerBoard, error) {
			// Return some test data
			return []model.ONUInfoPerBoard{
				{
					Board:        boardID,
					PON:          ponID,
					ID:           1,
					Name:         "Test ONU",
					OnuType:      "F670",
					SerialNumber: "SN001",
					RXPower:      "-20",
					Status:       "Online",
				},
			}, nil
		},
	}

	handler := NewOnuHandler(usecase)

	// Create request with context values
	req := httptest.NewRequest("GET", "/api/v1/board/1/pon/1", nil)
	ctx := context.WithValue(req.Context(), middleware.BoardIDKey, 1)
	ctx = context.WithValue(ctx, middleware.PonIDKey, 1)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.GetByBoardIDAndPonID(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %d", rr.Code)
	}
}

func TestOnuHandler_GetByBoardIDAndPonID_InvalidQuery(t *testing.T) {
	usecase := &mockOnuUsecase{}
	handler := NewOnuHandler(usecase)

	// Create request with invalid query parameter
	req := httptest.NewRequest("GET", "/api/v1/board/1/pon/1?invalid_param=value", nil)
	ctx := context.WithValue(req.Context(), middleware.BoardIDKey, 1)
	ctx = context.WithValue(ctx, middleware.PonIDKey, 1)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.GetByBoardIDAndPonID(rr, req)

	// Should return Bad Request
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status Bad Request, got %d", rr.Code)
	}
}

func TestOnuHandler_InterfaceCompliance(t *testing.T) {
	// Verify that OnuHandler implements OnuHandlerInterface
	var _ OnuHandlerInterface = NewOnuHandler(&mockOnuUsecase{})
	// The fact that this compiles confirms interface compliance
}

func TestOnuHandler_ContextValues(t *testing.T) {
	tests := []struct {
		name    string
		boardID int
		ponID   int
	}{
		{"Board 1 PON 1", 1, 1},
		{"Board 1 PON 8", 1, 8},
		{"Board 2 PON 16", 2, 16},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receivedBoardID := 0
			receivedPonID := 0

			usecase := &mockOnuUsecase{
				GetByBoardIDAndPonIDFunc: func(ctx context.Context, boardID, ponID int) ([]model.ONUInfoPerBoard, error) {
					receivedBoardID = boardID
					receivedPonID = ponID
					return []model.ONUInfoPerBoard{{Board: boardID, PON: ponID}}, nil
				},
			}

			handler := NewOnuHandler(usecase)

			req := httptest.NewRequest("GET", "/api/v1/board/1/pon/1", nil)
			ctx := context.WithValue(req.Context(), middleware.BoardIDKey, tt.boardID)
			ctx = context.WithValue(ctx, middleware.PonIDKey, tt.ponID)
			req = req.WithContext(ctx)

			rr := httptest.NewRecorder()
			handler.GetByBoardIDAndPonID(rr, req)

			if receivedBoardID != tt.boardID {
				t.Errorf("Expected boardID %d, got %d", tt.boardID, receivedBoardID)
			}

			if receivedPonID != tt.ponID {
				t.Errorf("Expected ponID %d, got %d", tt.ponID, receivedPonID)
			}
		})
	}
}

func TestOnuHandler_WithChiContext(t *testing.T) {
	usecase := &mockOnuUsecase{
		GetByBoardIDAndPonIDFunc: func(ctx context.Context, boardID, ponID int) ([]model.ONUInfoPerBoard, error) {
			return []model.ONUInfoPerBoard{{Board: boardID, PON: ponID}}, nil
		},
	}

	handler := NewOnuHandler(usecase)

	// Create request with chi context
	req := httptest.NewRequest("GET", "/api/v1/board/1/pon/1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("board_id", "1")
	rctx.URLParams.Add("pon_id", "1")

	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, middleware.BoardIDKey, 1)
	ctx = context.WithValue(ctx, middleware.PonIDKey, 1)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.GetByBoardIDAndPonID(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %d", rr.Code)
	}
}

func TestOnuHandler_EmptyResponse(t *testing.T) {
	usecase := &mockOnuUsecase{
		GetByBoardIDAndPonIDFunc: func(ctx context.Context, boardID, ponID int) ([]model.ONUInfoPerBoard, error) {
			// Return empty list
			return []model.ONUInfoPerBoard{}, nil
		},
	}

	handler := NewOnuHandler(usecase)

	req := httptest.NewRequest("GET", "/api/v1/board/1/pon/1", nil)
	ctx := context.WithValue(req.Context(), middleware.BoardIDKey, 1)
	ctx = context.WithValue(ctx, middleware.PonIDKey, 1)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.GetByBoardIDAndPonID(rr, req)

	// Should return Not Found for empty results
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status Not Found, got %d", rr.Code)
	}
}

func TestOnuHandler_GetByBoardIDPonIDAndOnuID_Success(t *testing.T) {
	expectedData := model.ONUCustomerInfo{
		Board:        1,
		PON:          1,
		ID:           5,
		Name:         "Customer A",
		OnuType:      "F670",
		SerialNumber: "ZTEGC123456",
		RXPower:      "-20.5",
		TXPower:      "2.5",
		Status:       "Online",
	}

	usecase := &mockOnuUsecase{
		GetByBoardIDPonIDAndOnuIDFunc: func(ctx context.Context, boardID, ponID, onuID int) (model.ONUCustomerInfo, error) {
			return expectedData, nil
		},
	}

	handler := NewOnuHandler(usecase)

	req := httptest.NewRequest("GET", "/api/v1/board/1/pon/1/onu/5", nil)
	ctx := context.WithValue(req.Context(), middleware.BoardIDKey, 1)
	ctx = context.WithValue(ctx, middleware.PonIDKey, 1)
	ctx = context.WithValue(ctx, middleware.OnuIDKey, 5)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.GetByBoardIDPonIDAndOnuID(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %d", rr.Code)
	}
}

func TestOnuHandler_GetByBoardIDPonIDAndOnuID_NotFound(t *testing.T) {
	usecase := &mockOnuUsecase{
		GetByBoardIDPonIDAndOnuIDFunc: func(ctx context.Context, boardID, ponID, onuID int) (model.ONUCustomerInfo, error) {
			// Return empty struct (all zeros)
			return model.ONUCustomerInfo{}, nil
		},
	}

	handler := NewOnuHandler(usecase)

	req := httptest.NewRequest("GET", "/api/v1/board/1/pon/1/onu/99", nil)
	ctx := context.WithValue(req.Context(), middleware.BoardIDKey, 1)
	ctx = context.WithValue(ctx, middleware.PonIDKey, 1)
	ctx = context.WithValue(ctx, middleware.OnuIDKey, 99)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.GetByBoardIDPonIDAndOnuID(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status Not Found, got %d", rr.Code)
	}
}

func TestOnuHandler_GetEmptyOnuID_Success(t *testing.T) {
	expectedData := []model.OnuID{
		{Board: 1, PON: 1, ID: 10},
		{Board: 1, PON: 1, ID: 20},
		{Board: 1, PON: 1, ID: 30},
	}

	usecase := &mockOnuUsecase{
		GetEmptyOnuIDFunc: func(ctx context.Context, boardID, ponID int) ([]model.OnuID, error) {
			return expectedData, nil
		},
	}

	handler := NewOnuHandler(usecase)

	req := httptest.NewRequest("GET", "/api/v1/board/1/pon/1/onu_id/empty", nil)
	ctx := context.WithValue(req.Context(), middleware.BoardIDKey, 1)
	ctx = context.WithValue(ctx, middleware.PonIDKey, 1)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.GetEmptyOnuID(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %d", rr.Code)
	}
}

func TestOnuHandler_GetOnuIDAndSerialNumber_Success(t *testing.T) {
	expectedData := []model.OnuSerialNumber{
		{Board: 1, PON: 1, ID: 1, SerialNumber: "ZTEGC111111"},
		{Board: 1, PON: 1, ID: 2, SerialNumber: "ZTEGC222222"},
	}

	usecase := &mockOnuUsecase{
		GetOnuIDAndSerialNumberFunc: func(ctx context.Context, boardID, ponID int) ([]model.OnuSerialNumber, error) {
			return expectedData, nil
		},
	}

	handler := NewOnuHandler(usecase)

	req := httptest.NewRequest("GET", "/api/v1/board/1/pon/1/onu_id_sn", nil)
	ctx := context.WithValue(req.Context(), middleware.BoardIDKey, 1)
	ctx = context.WithValue(ctx, middleware.PonIDKey, 1)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.GetOnuIDAndSerialNumber(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %d", rr.Code)
	}
}

func TestOnuHandler_UpdateEmptyOnuID_Success(t *testing.T) {
	usecase := &mockOnuUsecase{
		UpdateEmptyOnuIDFunc: func(ctx context.Context, boardID, ponID int) error {
			return nil
		},
	}

	handler := NewOnuHandler(usecase)

	req := httptest.NewRequest("GET", "/api/v1/board/1/pon/1/onu_id/update", nil)
	ctx := context.WithValue(req.Context(), middleware.BoardIDKey, 1)
	ctx = context.WithValue(ctx, middleware.PonIDKey, 1)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.UpdateEmptyOnuID(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %d", rr.Code)
	}
}

func TestOnuHandler_GetByBoardIDAndPonIDWithPaginate_Success(t *testing.T) {
	expectedData := []model.ONUInfoPerBoard{
		{Board: 1, PON: 1, ID: 1, Name: "ONU 1"},
		{Board: 1, PON: 1, ID: 2, Name: "ONU 2"},
	}

	usecase := &mockOnuUsecase{
		GetByBoardIDAndPonIDWithPaginationFunc: func(ctx context.Context, boardID, ponID, page, pageSize int) ([]model.ONUInfoPerBoard, int) {
			return expectedData, 10
		},
	}

	handler := NewOnuHandler(usecase)

	req := httptest.NewRequest("GET", "/api/v1/paginate/board/1/pon/1?page=1&page_size=10", nil)
	ctx := context.WithValue(req.Context(), middleware.BoardIDKey, 1)
	ctx = context.WithValue(ctx, middleware.PonIDKey, 1)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.GetByBoardIDAndPonIDWithPaginate(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %d", rr.Code)
	}
}

func TestOnuHandler_GetByBoardIDAndPonIDWithPaginate_NotFound(t *testing.T) {
	usecase := &mockOnuUsecase{
		GetByBoardIDAndPonIDWithPaginationFunc: func(ctx context.Context, boardID, ponID, page, pageSize int) ([]model.ONUInfoPerBoard, int) {
			// Return empty list
			return []model.ONUInfoPerBoard{}, 0
		},
	}

	handler := NewOnuHandler(usecase)

	req := httptest.NewRequest("GET", "/api/v1/paginate/board/1/pon/1?page=99&page_size=10", nil)
	ctx := context.WithValue(req.Context(), middleware.BoardIDKey, 1)
	ctx = context.WithValue(ctx, middleware.PonIDKey, 1)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.GetByBoardIDAndPonIDWithPaginate(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status Not Found, got %d", rr.Code)
	}
}

func TestOnuHandler_DeleteCache_Success(t *testing.T) {
	usecase := &mockOnuUsecase{
		DeleteCacheFunc: func(ctx context.Context, boardID, ponID int) error {
			return nil
		},
	}

	handler := NewOnuHandler(usecase)

	req := httptest.NewRequest("DELETE", "/api/v1/board/1/pon/1", nil)
	ctx := context.WithValue(req.Context(), middleware.BoardIDKey, 1)
	ctx = context.WithValue(ctx, middleware.PonIDKey, 1)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.DeleteCache(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %d", rr.Code)
	}
}

func TestOnuHandler_GetByBoardIDAndPonID_Error(t *testing.T) {
	usecase := &mockOnuUsecase{
		GetByBoardIDAndPonIDFunc: func(ctx context.Context, boardID, ponID int) ([]model.ONUInfoPerBoard, error) {
			return nil, errors.New("SNMP error")
		},
	}

	handler := NewOnuHandler(usecase)

	req := httptest.NewRequest("GET", "/api/v1/board/1/pon/1", nil)
	ctx := context.WithValue(req.Context(), middleware.BoardIDKey, 1)
	ctx = context.WithValue(ctx, middleware.PonIDKey, 1)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.GetByBoardIDAndPonID(rr, req)

	if rr.Code == http.StatusOK {
		t.Error("Expected error status, got OK")
	}
}

func TestOnuHandler_GetByBoardIDPonIDAndOnuID_Error(t *testing.T) {
	usecase := &mockOnuUsecase{
		GetByBoardIDPonIDAndOnuIDFunc: func(ctx context.Context, boardID, ponID, onuID int) (model.ONUCustomerInfo, error) {
			return model.ONUCustomerInfo{}, errors.New("SNMP error")
		},
	}

	handler := NewOnuHandler(usecase)

	req := httptest.NewRequest("GET", "/api/v1/board/1/pon/1/onu/5", nil)
	ctx := context.WithValue(req.Context(), middleware.BoardIDKey, 1)
	ctx = context.WithValue(ctx, middleware.PonIDKey, 1)
	ctx = context.WithValue(ctx, middleware.OnuIDKey, 5)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.GetByBoardIDPonIDAndOnuID(rr, req)

	if rr.Code == http.StatusOK {
		t.Error("Expected error status, got OK")
	}
}

func TestOnuHandler_GetEmptyOnuID_Error(t *testing.T) {
	usecase := &mockOnuUsecase{
		GetEmptyOnuIDFunc: func(ctx context.Context, boardID, ponID int) ([]model.OnuID, error) {
			return nil, errors.New("SNMP error")
		},
	}

	handler := NewOnuHandler(usecase)

	req := httptest.NewRequest("GET", "/api/v1/board/1/pon/1/onu_id/empty", nil)
	ctx := context.WithValue(req.Context(), middleware.BoardIDKey, 1)
	ctx = context.WithValue(ctx, middleware.PonIDKey, 1)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.GetEmptyOnuID(rr, req)

	if rr.Code == http.StatusOK {
		t.Error("Expected error status, got OK")
	}
}

func TestOnuHandler_GetOnuIDAndSerialNumber_Error(t *testing.T) {
	usecase := &mockOnuUsecase{
		GetOnuIDAndSerialNumberFunc: func(ctx context.Context, boardID, ponID int) ([]model.OnuSerialNumber, error) {
			return nil, errors.New("SNMP error")
		},
	}

	handler := NewOnuHandler(usecase)

	req := httptest.NewRequest("GET", "/api/v1/board/1/pon/1/onu_id_sn", nil)
	ctx := context.WithValue(req.Context(), middleware.BoardIDKey, 1)
	ctx = context.WithValue(ctx, middleware.PonIDKey, 1)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.GetOnuIDAndSerialNumber(rr, req)

	if rr.Code == http.StatusOK {
		t.Error("Expected error status, got OK")
	}
}

func TestOnuHandler_UpdateEmptyOnuID_Error(t *testing.T) {
	usecase := &mockOnuUsecase{
		UpdateEmptyOnuIDFunc: func(ctx context.Context, boardID, ponID int) error {
			return errors.New("Redis error")
		},
	}

	handler := NewOnuHandler(usecase)

	req := httptest.NewRequest("GET", "/api/v1/board/1/pon/1/onu_id/update", nil)
	ctx := context.WithValue(req.Context(), middleware.BoardIDKey, 1)
	ctx = context.WithValue(ctx, middleware.PonIDKey, 1)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.UpdateEmptyOnuID(rr, req)

	if rr.Code == http.StatusOK {
		t.Error("Expected error status, got OK")
	}
}

func TestOnuHandler_DeleteCache_Error(t *testing.T) {
	usecase := &mockOnuUsecase{
		DeleteCacheFunc: func(ctx context.Context, boardID, ponID int) error {
			return errors.New("Redis error")
		},
	}

	handler := NewOnuHandler(usecase)

	req := httptest.NewRequest("DELETE", "/api/v1/board/1/pon/1", nil)
	ctx := context.WithValue(req.Context(), middleware.BoardIDKey, 1)
	ctx = context.WithValue(ctx, middleware.PonIDKey, 1)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.DeleteCache(rr, req)

	if rr.Code == http.StatusOK {
		t.Error("Expected error status, got OK")
	}
}
