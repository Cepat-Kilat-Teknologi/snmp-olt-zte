package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/utils"
	"github.com/go-chi/chi/v5"
)

func TestGetBoardID(t *testing.T) {
	tests := []struct {
		name      string
		ctx       context.Context
		wantValue int
		wantOk    bool
	}{
		{
			name:      "Valid board ID in context",
			ctx:       context.WithValue(context.Background(), BoardIDKey, 1),
			wantValue: 1,
			wantOk:    true,
		},
		{
			name:      "Board ID 2 in context",
			ctx:       context.WithValue(context.Background(), BoardIDKey, 2),
			wantValue: 2,
			wantOk:    true,
		},
		{
			name:      "No board ID in context",
			ctx:       context.Background(),
			wantValue: 0,
			wantOk:    false,
		},
		{
			name:      "Wrong type in context",
			ctx:       context.WithValue(context.Background(), BoardIDKey, "not-an-int"),
			wantValue: 0,
			wantOk:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, gotOk := GetBoardID(tt.ctx)
			if gotValue != tt.wantValue {
				t.Errorf("GetBoardID() value = %v, want %v", gotValue, tt.wantValue)
			}
			if gotOk != tt.wantOk {
				t.Errorf("GetBoardID() ok = %v, want %v", gotOk, tt.wantOk)
			}
		})
	}
}

func TestGetPonID(t *testing.T) {
	tests := []struct {
		name      string
		ctx       context.Context
		wantValue int
		wantOk    bool
	}{
		{
			name:      "Valid pon ID in context",
			ctx:       context.WithValue(context.Background(), PonIDKey, 8),
			wantValue: 8,
			wantOk:    true,
		},
		{
			name:      "Pon ID 16 in context",
			ctx:       context.WithValue(context.Background(), PonIDKey, 16),
			wantValue: 16,
			wantOk:    true,
		},
		{
			name:      "No pon ID in context",
			ctx:       context.Background(),
			wantValue: 0,
			wantOk:    false,
		},
		{
			name:      "Wrong type in context",
			ctx:       context.WithValue(context.Background(), PonIDKey, "not-an-int"),
			wantValue: 0,
			wantOk:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, gotOk := GetPonID(tt.ctx)
			if gotValue != tt.wantValue {
				t.Errorf("GetPonID() value = %v, want %v", gotValue, tt.wantValue)
			}
			if gotOk != tt.wantOk {
				t.Errorf("GetPonID() ok = %v, want %v", gotOk, tt.wantOk)
			}
		})
	}
}

func TestGetOnuID(t *testing.T) {
	tests := []struct {
		name      string
		ctx       context.Context
		wantValue int
		wantOk    bool
	}{
		{
			name:      "Valid ONU ID in context",
			ctx:       context.WithValue(context.Background(), OnuIDKey, 64),
			wantValue: 64,
			wantOk:    true,
		},
		{
			name:      "ONU ID 128 in context",
			ctx:       context.WithValue(context.Background(), OnuIDKey, 128),
			wantValue: 128,
			wantOk:    true,
		},
		{
			name:      "No ONU ID in context",
			ctx:       context.Background(),
			wantValue: 0,
			wantOk:    false,
		},
		{
			name:      "Wrong type in context",
			ctx:       context.WithValue(context.Background(), OnuIDKey, "not-an-int"),
			wantValue: 0,
			wantOk:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, gotOk := GetOnuID(tt.ctx)
			if gotValue != tt.wantValue {
				t.Errorf("GetOnuID() value = %v, want %v", gotValue, tt.wantValue)
			}
			if gotOk != tt.wantOk {
				t.Errorf("GetOnuID() ok = %v, want %v", gotOk, tt.wantOk)
			}
		})
	}
}

func TestValidateBoardPonParams(t *testing.T) {
	tests := []struct {
		name           string
		boardID        string
		ponID          string
		expectedStatus int
		expectError    bool
	}{
		{
			name:           "Valid board 1 pon 1",
			boardID:        "1",
			ponID:          "1",
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "Valid board 2 pon 16",
			boardID:        "2",
			ponID:          "16",
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "Invalid board 0",
			boardID:        "0",
			ponID:          "1",
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name:           "Invalid board 3",
			boardID:        "3",
			ponID:          "1",
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name:           "Invalid board non-numeric",
			boardID:        "abc",
			ponID:          "1",
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name:           "Invalid pon 0",
			boardID:        "1",
			ponID:          "0",
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name:           "Invalid pon 17",
			boardID:        "1",
			ponID:          "17",
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name:           "Invalid pon non-numeric",
			boardID:        "1",
			ponID:          "xyz",
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name:           "Valid board 1 pon 8",
			boardID:        "1",
			ponID:          "8",
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "Valid board 2 pon 10",
			boardID:        "2",
			ponID:          "10",
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test handler that checks context values
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// If we get here, validation passed
				boardIDInt := r.Context().Value(BoardIDKey)
				ponIDInt := r.Context().Value(PonIDKey)

				if boardIDInt == nil || ponIDInt == nil {
					t.Error("Expected boardID and ponID in context")
				}

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("OK"))
			})

			// Wrap with validation middleware (C320 default slots {1,2}, 16 PONs each)
			validate := ValidateBoardPonParams(map[int]int{1: 16, 2: 16})
			handler := validate(testHandler)

			// Create a chi router to set URL params
			r := chi.NewRouter()
			r.With(validate).Get("/board/{board_id}/pon/{pon_id}", testHandler)

			// Create request
			req := httptest.NewRequest("GET", "/board/"+tt.boardID+"/pon/"+tt.ponID, nil)

			// Set chi URL params manually
			rctx := chi.NewRouteContext()

			// Set chi URL params manually
			rctx.URLParams.Add("board_id", tt.boardID)

			// Set chi URL params manually
			rctx.URLParams.Add("pon_id", tt.ponID)

			// Set chi context
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			// Create recorder
			rr := httptest.NewRecorder()

			// Execute request
			handler.ServeHTTP(rr, req)

			// Check status code
			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("Expected status %v, got %v", tt.expectedStatus, status)
			}

			// If error expected, check the response
			if tt.expectError {
				var response utils.ErrorResponse
				if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
					t.Errorf("Failed to decode error response: %v", err)
				}

				if response.Code != http.StatusBadRequest {
					t.Errorf("Expected error code %v, got %v", http.StatusBadRequest, response.Code)
				}
			}
		})
	}
}

func TestValidateOnuIDParam(t *testing.T) {
	tests := []struct {
		name           string
		onuID          string
		expectedStatus int
		expectError    bool
	}{
		{
			name:           "Valid ONU ID 1",
			onuID:          "1",
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "Valid ONU ID 64",
			onuID:          "64",
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "Valid ONU ID 128",
			onuID:          "128",
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "Invalid ONU ID 0",
			onuID:          "0",
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name:           "Invalid ONU ID 129",
			onuID:          "129",
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name:           "Invalid ONU ID non-numeric",
			onuID:          "abc",
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name:           "Invalid ONU ID negative",
			onuID:          "-5",
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test handler
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// If we get here, validation passed
				onuIDInt := r.Context().Value(OnuIDKey)

				if onuIDInt == nil {
					t.Error("Expected onuID in context")
				}

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("OK"))
			})

			// Wrap with validation middleware
			handler := ValidateOnuIDParam(testHandler)

			// Create a request with chi context
			req := httptest.NewRequest("GET", "/onu/"+tt.onuID, nil)

			// Set chi URL params manually
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("onu_id", tt.onuID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			rr := httptest.NewRecorder()

			// Execute request
			handler.ServeHTTP(rr, req)

			// Check status code
			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("Expected status %v, got %v", tt.expectedStatus, status)
			}

			// If error expected, check the response
			if tt.expectError {
				var response utils.ErrorResponse
				if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
					t.Errorf("Failed to decode error response: %v", err)
				}

				if response.Code != http.StatusBadRequest {
					t.Errorf("Expected error code %v, got %v", http.StatusBadRequest, response.Code)
				}
			}
		})
	}
}
