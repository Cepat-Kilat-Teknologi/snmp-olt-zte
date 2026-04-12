package utils

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	apperrors "github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/errors"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/model"
)

// newTestRequest creates a plain GET request for use as the `r` argument
// to error helpers in tests. No request ID is attached.
func newTestRequest() *http.Request {
	return httptest.NewRequest(http.MethodGet, "/test", nil)
}

func TestSendJSONResponse(t *testing.T) {
	// Initiate ResponseWriter dan Request
	rr := httptest.NewRecorder()

	response := model.OnuID{
		Board: 2,
		PON:   8,
		ID:    1,
	}

	// Call the SendJSONResponse function
	SendJSONResponse(rr, http.StatusOK, response)

	// Check the status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Status code tidak sesuai: got %v want %v", status, http.StatusOK)
	}

	// Check the content type
	expectedContentType := "application/json"
	if contentType := rr.Header().Get("Content-Type"); contentType != expectedContentType {
		t.Errorf("Content-Type tidak sesuai: got %v want %v", contentType, expectedContentType)
	}

	// Periksa Body Response
	var decodedResponse model.OnuID
	err := json.NewDecoder(rr.Body).Decode(&decodedResponse)
	if err != nil {
		t.Errorf("Gagal mendekode respons JSON: %v", err)
	}

	// Uji kasus di mana encoding JSON gagal
	rrError := httptest.NewRecorder()
	errorResponse := make(chan int) // channels cannot be JSON-encoded
	SendJSONResponse(rrError, http.StatusOK, errorResponse)

	if status := rrError.Code; status != http.StatusOK {
		t.Errorf("Status code tidak sesuai: got %v want %v", status, http.StatusOK)
	}

	expectedContentTypeError := "application/json"
	if contentType := rrError.Header().Get("Content-Type"); contentType != expectedContentTypeError {
		t.Errorf("Content-Type tidak sesuai: got %v want %v", contentType, expectedContentTypeError)
	}

	if body := rrError.Body.String(); body != "" {
		t.Errorf("Response body harus kosong jika encoding JSON gagal: got %v", body)
	}
}

func TestErrorBadRequest(t *testing.T) {
	rr := httptest.NewRecorder()
	err := errors.New("Bad Request Error")
	ErrorBadRequest(rr, newTestRequest(), err)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("Status code tidak sesuai: got %v want %v", status, http.StatusBadRequest)
	}

	expectedContentType := "application/json"
	if contentType := rr.Header().Get("Content-Type"); contentType != expectedContentType {
		t.Errorf("Content-Type tidak sesuai: got %v want %v", contentType, expectedContentType)
	}

	var response ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Errorf("Gagal mendecode respons JSON: %v", err)
	}

	if response.Code != http.StatusBadRequest || response.Status != "Bad Request" || response.Data != err.Error() {
		t.Errorf("Respons JSON tidak sesuai: %+v", response)
	}
}

func TestErrorInternalServerError(t *testing.T) {
	rr := httptest.NewRecorder()
	err := errors.New("Internal Server Error")
	ErrorInternalServerError(rr, newTestRequest(), err)

	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("Status code tidak sesuai: got %v want %v", status, http.StatusInternalServerError)
	}

	expectedContentType := "application/json"
	if contentType := rr.Header().Get("Content-Type"); contentType != expectedContentType {
		t.Errorf("Content-Type tidak sesuai: got %v want %v", contentType, expectedContentType)
	}

	var response ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Errorf("Gagal mendecode respons JSON: %v", err)
	}

	if response.Code != http.StatusInternalServerError || response.Status != "Internal Server Error" || response.Data != err.Error() {
		t.Errorf("Respons JSON tidak sesuai: %+v", response)
	}
}

func TestErrorNotFound(t *testing.T) {
	rr := httptest.NewRecorder()
	err := errors.New("Not Found Error")
	ErrorNotFound(rr, newTestRequest(), err)

	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("Status code tidak sesuai: got %v want %v", status, http.StatusNotFound)
	}

	expectedContentType := "application/json"
	if contentType := rr.Header().Get("Content-Type"); contentType != expectedContentType {
		t.Errorf("Content-Type tidak sesuai: got %v want %v", contentType, expectedContentType)
	}

	var response ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Errorf("Gagal mendecode respons JSON: %v", err)
	}

	if response.Code != http.StatusNotFound || response.Status != "Not Found" || response.Data != err.Error() {
		t.Errorf("Respons JSON tidak sesuai: %+v", response)
	}
}

func TestHandleError_ValidationError(t *testing.T) {
	rr := httptest.NewRecorder()
	appErr := apperrors.NewValidationError("board_id must be 1 or 2",
		map[string]interface{}{"received": "3"})

	HandleError(rr, newTestRequest(), appErr)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("Status code tidak sesuai: got %v want %v", status, http.StatusBadRequest)
	}

	expectedContentType := "application/json"
	if contentType := rr.Header().Get("Content-Type"); contentType != expectedContentType {
		t.Errorf("Content-Type tidak sesuai: got %v want %v", contentType, expectedContentType)
	}

	var response ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Errorf("Gagal mendecode respons JSON: %v", err)
	}

	if response.Code != http.StatusBadRequest {
		t.Errorf("Response code tidak sesuai: got %v want %v", response.Code, http.StatusBadRequest)
	}
	if response.ErrorCode != string(apperrors.ErrorTypeValidation) {
		t.Errorf("ErrorCode tidak sesuai: got %v want %v", response.ErrorCode, apperrors.ErrorTypeValidation)
	}
}

func TestHandleError_NotFoundError(t *testing.T) {
	rr := httptest.NewRecorder()
	appErr := apperrors.NewNotFoundError("ONU info",
		map[string]int{"board_id": 1, "pon_id": 5})

	HandleError(rr, newTestRequest(), appErr)

	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("Status code tidak sesuai: got %v want %v", status, http.StatusNotFound)
	}

	var response ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Errorf("Gagal mendecode respons JSON: %v", err)
	}

	if response.Code != http.StatusNotFound {
		t.Errorf("Response code tidak sesuai: got %v want %v", response.Code, http.StatusNotFound)
	}
	if response.ErrorCode != string(apperrors.ErrorTypeNotFound) {
		t.Errorf("ErrorCode tidak sesuai: got %v want %v", response.ErrorCode, apperrors.ErrorTypeNotFound)
	}
}

func TestHandleError_SNMPError(t *testing.T) {
	rr := httptest.NewRecorder()
	appErr := apperrors.NewSNMPError("Get", errors.New("timeout"))

	HandleError(rr, newTestRequest(), appErr)

	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("Status code tidak sesuai: got %v want %v", status, http.StatusInternalServerError)
	}

	var response ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Errorf("Gagal mendecode respons JSON: %v", err)
	}

	if response.Code != http.StatusInternalServerError {
		t.Errorf("Response code tidak sesuai: got %v want %v", response.Code, http.StatusInternalServerError)
	}
	if response.ErrorCode != string(apperrors.ErrorTypeSNMP) {
		t.Errorf("ErrorCode tidak sesuai: got %v want %v", response.ErrorCode, apperrors.ErrorTypeSNMP)
	}
}

func TestHandleError_RedisError(t *testing.T) {
	rr := httptest.NewRecorder()
	appErr := apperrors.NewRedisError("Get", errors.New("connection refused"))

	HandleError(rr, newTestRequest(), appErr)

	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("Status code tidak sesuai: got %v want %v", status, http.StatusInternalServerError)
	}
}

func TestHandleError_InternalError(t *testing.T) {
	rr := httptest.NewRecorder()
	appErr := apperrors.NewInternalError("failed to unmarshal", errors.New("invalid JSON"))

	HandleError(rr, newTestRequest(), appErr)

	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("Status code tidak sesuai: got %v want %v", status, http.StatusInternalServerError)
	}
}

func TestHandleError_ConfigError(t *testing.T) {
	rr := httptest.NewRecorder()
	appErr := apperrors.NewConfigError("invalid configuration", errors.New("missing field"))

	HandleError(rr, newTestRequest(), appErr)

	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("Status code tidak sesuai: got %v want %v", status, http.StatusInternalServerError)
	}
}

func TestHandleError_UnknownErrorType(t *testing.T) {
	rr := httptest.NewRecorder()
	appErr := &apperrors.AppError{
		Type:    "UNKNOWN_TYPE",
		Message: "unknown error",
	}

	HandleError(rr, newTestRequest(), appErr)

	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("Status code tidak sesuai: got %v want %v", status, http.StatusInternalServerError)
	}
}

func TestHandleError_NonAppError(t *testing.T) {
	rr := httptest.NewRecorder()
	err := errors.New("standard go error")

	HandleError(rr, newTestRequest(), err)

	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("Status code tidak sesuai: got %v want %v", status, http.StatusInternalServerError)
	}

	var response ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Errorf("Gagal mendecode respons JSON: %v", err)
	}

	if response.Code != http.StatusInternalServerError {
		t.Errorf("Response code tidak sesuai: got %v want %v", response.Code, http.StatusInternalServerError)
	}
	// Non-AppError should fall back to INTERNAL_ERROR code
	if response.ErrorCode != string(apperrors.ErrorTypeInternal) {
		t.Errorf("ErrorCode tidak sesuai: got %v want %v", response.ErrorCode, apperrors.ErrorTypeInternal)
	}
}

func TestHandleError_NilRequest(t *testing.T) {
	// Passing nil r is supported — the helpers must not panic and should
	// simply omit request_id from the response body.
	rr := httptest.NewRecorder()
	HandleError(rr, nil, errors.New("boom"))
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 for nil request path, got %d", rr.Code)
	}
	var response ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if response.RequestID != "" {
		t.Errorf("Expected empty request_id when r is nil, got %q", response.RequestID)
	}
}

func TestHandleError_RequestIDPropagated(t *testing.T) {
	// Create a request with a request ID in context
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	ctx := context.WithValue(req.Context(), RequestIDKey, "req-abc-123")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	HandleError(rr, req, apperrors.NewValidationError("bad", nil))

	var response ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if response.RequestID != "req-abc-123" {
		t.Errorf("RequestID not propagated: got %q want %q", response.RequestID, "req-abc-123")
	}
}
