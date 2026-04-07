package utils

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	apperrors "github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/errors"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/model"
)

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
	// Inisialisasi ResponseWriter yang akan selalu gagal saat encoding JSON
	rrError := httptest.NewRecorder()
	// Sebagai contoh, gunakan objek yang tidak dapat di-encode sebagai respons
	errorResponse := make(chan int) // Ini akan gagal saat encoding JSON
	SendJSONResponse(rrError, http.StatusOK, errorResponse)

	// Periksa kode status respons
	if status := rrError.Code; status != http.StatusOK {
		t.Errorf("Status code tidak sesuai: got %v want %v", status, http.StatusOK)
	}

	// Periksa tipe konten
	expectedContentTypeError := "application/json"
	if contentType := rrError.Header().Get("Content-Type"); contentType != expectedContentTypeError {
		t.Errorf("Content-Type tidak sesuai: got %v want %v", contentType, expectedContentTypeError)
	}

	// Pastikan bahwa response body kosong karena encoding JSON gagal
	if body := rrError.Body.String(); body != "" {
		t.Errorf("Response body harus kosong jika encoding JSON gagal: got %v", body)
	}

}

func TestErrorBadRequest(t *testing.T) {
	rr := httptest.NewRecorder()
	err := errors.New("Bad Request Error")
	ErrorBadRequest(rr, err)

	// Periksa kode status respons
	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("Status code tidak sesuai: got %v want %v", status, http.StatusBadRequest)
	}

	// Periksa tipe konten
	expectedContentType := "application/json"
	if contentType := rr.Header().Get("Content-Type"); contentType != expectedContentType {
		t.Errorf("Content-Type tidak sesuai: got %v want %v", contentType, expectedContentType)
	}

	// Periksa pesan kesalahan dalam respons JSON
	var response ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Errorf("Gagal mendecode respons JSON: %v", err)
	}

	if response.Code != http.StatusBadRequest || response.Status != "Bad Request" || response.Error.Message != err.Error() {
		t.Errorf("Respons JSON tidak sesuai")
	}
}

func TestErrorInternalServerError(t *testing.T) {
	rr := httptest.NewRecorder()
	err := errors.New("Internal Server Error")
	ErrorInternalServerError(rr, err)

	// Periksa kode status respons
	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("Status code tidak sesuai: got %v want %v", status, http.StatusInternalServerError)
	}

	// Periksa tipe konten
	expectedContentType := "application/json"
	if contentType := rr.Header().Get("Content-Type"); contentType != expectedContentType {
		t.Errorf("Content-Type tidak sesuai: got %v want %v", contentType, expectedContentType)
	}

	// Periksa pesan kesalahan dalam respons JSON
	var response ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Errorf("Gagal mendecode respons JSON: %v", err)
	}

	if response.Code != http.StatusInternalServerError || response.Status != "Internal Server Error" || response.Error.Message != err.Error() {
		t.Errorf("Respons JSON tidak sesuai")
	}
}

func TestErrorNotFound(t *testing.T) {
	rr := httptest.NewRecorder()
	err := errors.New("Not Found Error")
	ErrorNotFound(rr, err)

	// Periksa kode status respons
	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("Status code tidak sesuai: got %v want %v", status, http.StatusNotFound)
	}

	// Periksa tipe konten
	expectedContentType := "application/json"
	if contentType := rr.Header().Get("Content-Type"); contentType != expectedContentType {
		t.Errorf("Content-Type tidak sesuai: got %v want %v", contentType, expectedContentType)
	}

	// Periksa pesan kesalahan dalam respons JSON
	var response ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Errorf("Gagal mendecode respons JSON: %v", err)
	}

	if response.Code != http.StatusNotFound || response.Status != "Not Found" || response.Error.Message != err.Error() {
		t.Errorf("Respons JSON tidak sesuai")
	}
}

func TestHandleError_ValidationError(t *testing.T) {
	rr := httptest.NewRecorder()
	appErr := apperrors.NewValidationError("board_id must be 1 or 2",
		map[string]interface{}{"received": "3"})

	HandleError(rr, appErr)

	// Check status code
	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("Status code tidak sesuai: got %v want %v", status, http.StatusBadRequest)
	}

	// Check content type
	expectedContentType := "application/json"
	if contentType := rr.Header().Get("Content-Type"); contentType != expectedContentType {
		t.Errorf("Content-Type tidak sesuai: got %v want %v", contentType, expectedContentType)
	}

	// Check response body
	var response ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Errorf("Gagal mendecode respons JSON: %v", err)
	}

	if response.Code != http.StatusBadRequest {
		t.Errorf("Response code tidak sesuai: got %v want %v", response.Code, http.StatusBadRequest)
	}
}

func TestHandleError_NotFoundError(t *testing.T) {
	rr := httptest.NewRecorder()
	appErr := apperrors.NewNotFoundError("ONU info",
		map[string]int{"board_id": 1, "pon_id": 5})

	HandleError(rr, appErr)

	// Check status code
	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("Status code tidak sesuai: got %v want %v", status, http.StatusNotFound)
	}

	// Check response
	var response ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Errorf("Gagal mendecode respons JSON: %v", err)
	}

	if response.Code != http.StatusNotFound {
		t.Errorf("Response code tidak sesuai: got %v want %v", response.Code, http.StatusNotFound)
	}
}

func TestHandleError_SNMPError(t *testing.T) {
	rr := httptest.NewRecorder()
	appErr := apperrors.NewSNMPError("Get", errors.New("timeout"))

	HandleError(rr, appErr)

	// Check status code
	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("Status code tidak sesuai: got %v want %v", status, http.StatusInternalServerError)
	}

	// Check response
	var response ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Errorf("Gagal mendecode respons JSON: %v", err)
	}

	if response.Code != http.StatusInternalServerError {
		t.Errorf("Response code tidak sesuai: got %v want %v", response.Code, http.StatusInternalServerError)
	}
}

func TestHandleError_RedisError(t *testing.T) {
	rr := httptest.NewRecorder()
	appErr := apperrors.NewRedisError("Get", errors.New("connection refused"))

	HandleError(rr, appErr)

	// Check status code
	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("Status code tidak sesuai: got %v want %v", status, http.StatusInternalServerError)
	}
}

func TestHandleError_InternalError(t *testing.T) {
	rr := httptest.NewRecorder()
	appErr := apperrors.NewInternalError("failed to unmarshal", errors.New("invalid JSON"))

	HandleError(rr, appErr)

	// Check status code
	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("Status code tidak sesuai: got %v want %v", status, http.StatusInternalServerError)
	}
}

func TestHandleError_ConfigError(t *testing.T) {
	rr := httptest.NewRecorder()
	appErr := apperrors.NewConfigError("invalid configuration", errors.New("missing field"))

	HandleError(rr, appErr)

	// Check status code
	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("Status code tidak sesuai: got %v want %v", status, http.StatusInternalServerError)
	}
}

func TestHandleError_UnknownErrorType(t *testing.T) {
	rr := httptest.NewRecorder()
	// Create AppError with unknown type
	appErr := &apperrors.AppError{
		Type:    "UNKNOWN_TYPE",
		Message: "unknown error",
	}

	HandleError(rr, appErr)

	// Should default to 500
	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("Status code tidak sesuai: got %v want %v", status, http.StatusInternalServerError)
	}
}

func TestHandleError_NonAppError(t *testing.T) {
	rr := httptest.NewRecorder()
	err := errors.New("standard go error")

	HandleError(rr, err)

	// Should default to 500
	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("Status code tidak sesuai: got %v want %v", status, http.StatusInternalServerError)
	}

	// Check response
	var response ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Errorf("Gagal mendecode respons JSON: %v", err)
	}

	if response.Code != http.StatusInternalServerError {
		t.Errorf("Response code tidak sesuai: got %v want %v", response.Code, http.StatusInternalServerError)
	}
}
