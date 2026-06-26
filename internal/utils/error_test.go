package utils

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"syscall"
	"testing"

	apperrors "github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/errors"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/model"
)

// newTestRequest creates a plain GET request for use as the `r` argument
// to error helpers in tests. No request ID is attached.
func newTestRequest() *http.Request {
	return httptest.NewRequest(http.MethodGet, "/test", nil)
}

// timeoutErr is a minimal net.Error whose Timeout() reports true, used to
// exercise the typed (non-string) device-unreachable detection path.
type timeoutErr struct{}

func (timeoutErr) Error() string   { return "i/o timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

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

func TestErrorUnauthorized(t *testing.T) {
	rr := httptest.NewRecorder()
	err := apperrors.NewUnauthorizedError("invalid or missing API key")
	ErrorUnauthorized(rr, newTestRequest(), err)

	if status := rr.Code; status != http.StatusUnauthorized {
		t.Errorf("Status code tidak sesuai: got %v want %v", status, http.StatusUnauthorized)
	}
	var response ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Errorf("Gagal mendecode respons JSON: %v", err)
	}
	if response.Code != http.StatusUnauthorized || response.Status != "Unauthorized" {
		t.Errorf("Respons JSON tidak sesuai: %+v", response)
	}
	if response.ErrorCode != string(apperrors.ErrorTypeUnauthorized) {
		t.Errorf("ErrorCode tidak sesuai: got %v want %v", response.ErrorCode, apperrors.ErrorTypeUnauthorized)
	}
}

func TestHandleError_Unauthorized(t *testing.T) {
	rr := httptest.NewRecorder()
	HandleError(rr, newTestRequest(), apperrors.NewUnauthorizedError("nope"))

	if status := rr.Code; status != http.StatusUnauthorized {
		t.Errorf("Status code tidak sesuai: got %v want %v", status, http.StatusUnauthorized)
	}
	var response ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Errorf("Gagal mendecode respons JSON: %v", err)
	}
	if response.ErrorCode != string(apperrors.ErrorTypeUnauthorized) {
		t.Errorf("ErrorCode tidak sesuai: got %v want %v", response.ErrorCode, apperrors.ErrorTypeUnauthorized)
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

// TestHandleError_SNMPError covers a genuine internal SNMP fault: the OLT was
// reachable but returned an unusable response. This is the service's own
// problem and must stay HTTP 500 with error_code SNMP_ERROR.
func TestHandleError_SNMPError(t *testing.T) {
	rr := httptest.NewRecorder()
	appErr := apperrors.NewSNMPError("Get", errors.New("no variables in response"))

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

// TestHandleError_SNMPError_DeviceUnreachable is the core regression for the
// 500->503 fix: when the OLT cannot be reached over SNMP, the adapter must
// return HTTP 503 with a SERVICE_UNAVAILABLE-class error_code (a
// dependency-unreachable condition, not the service's own internal fault).
// The cases mirror what gosnmp surfaces when the OLT is down/unreachable.
func TestHandleError_SNMPError_DeviceUnreachable(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"connection refused", errors.New("error reading from socket: read udp 10.0.0.2:50000->10.0.0.1:161: recvfrom: connection refused")},
		{"i/o timeout", errors.New("error reading from socket: read udp 10.0.0.2:50000->10.0.0.1:161: i/o timeout")},
		{"request timeout", errors.New("request timeout (after 3 retries)")},
		{"no route to host", errors.New("dial udp 10.0.0.1:161: connect: no route to host")},
		{"typed net timeout", &net.OpError{Op: "read", Net: "udp", Err: timeoutErr{}}},
		{"typed ECONNREFUSED", &net.OpError{Op: "read", Net: "udp", Err: syscall.ECONNREFUSED}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			// Mirror the production call site: usecase wraps the SNMP client
			// transport error via NewSNMPError("walk", err).
			appErr := apperrors.NewSNMPError("walk", tc.err)

			HandleError(rr, newTestRequest(), appErr)

			if rr.Code != http.StatusServiceUnavailable {
				t.Fatalf("Status code: got %d want %d (%s)", rr.Code, http.StatusServiceUnavailable, rr.Body.String())
			}

			var response ErrorResponse
			if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
				t.Fatalf("decode: %v", err)
			}
			// Both the envelope `code` field and the real HTTP status are 503.
			if response.Code != http.StatusServiceUnavailable {
				t.Errorf("envelope code: got %d want %d", response.Code, http.StatusServiceUnavailable)
			}
			if response.Status != "Service Unavailable" {
				t.Errorf("status: got %q want %q", response.Status, "Service Unavailable")
			}
			if response.ErrorCode != string(apperrors.ErrorTypeServiceUnavailable) {
				t.Errorf("error_code: got %q want %q", response.ErrorCode, apperrors.ErrorTypeServiceUnavailable)
			}
		})
	}
}

// TestHandleError_ServiceUnavailableType verifies the explicit
// SERVICE_UNAVAILABLE AppError type also maps to HTTP 503.
func TestHandleError_ServiceUnavailableType(t *testing.T) {
	rr := httptest.NewRecorder()
	appErr := &apperrors.AppError{
		Type:    apperrors.ErrorTypeServiceUnavailable,
		Message: "OLT unreachable",
		Err:     errors.New("connection refused"),
	}

	HandleError(rr, newTestRequest(), appErr)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("Status code: got %d want %d", rr.Code, http.StatusServiceUnavailable)
	}
	var response ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if response.ErrorCode != string(apperrors.ErrorTypeServiceUnavailable) {
		t.Errorf("error_code: got %q want %q", response.ErrorCode, apperrors.ErrorTypeServiceUnavailable)
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
