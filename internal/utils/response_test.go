package utils

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendRequestJSONResponse(t *testing.T) {
	// Initialising ResponseWriter dan Request
	rr := httptest.NewRecorder()

	// Response for 200 OK
	response := WebResponse{
		Code:   200,
		Status: "OK",
		Data:   map[string]string{"key": "value"},
	}

	// Call function SendJSONResponse
	SendJSONResponse(rr, http.StatusOK, response)

	// Check status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Status code tidak sesuai: got %v want %v", status, http.StatusOK)
	}

	// Check content type
	expectedContentType := "application/json"
	if contentType := rr.Header().Get("Content-Type"); contentType != expectedContentType {
		t.Errorf("Content-Type tidak sesuai: got %v want %v", contentType, expectedContentType)
	}

	// Check respons JSON
	var decodedResponse WebResponse
	err := json.NewDecoder(rr.Body).Decode(&decodedResponse)
	if err != nil {
		t.Errorf("Gagal mendekode respons JSON: %v", err)
	}

}

func TestErrorBadRequestBos(t *testing.T) {
	// Initialising ResponseWriter
	rr := httptest.NewRecorder()

	// Cont error
	err := errors.New("Bad Request")

	// Call function ErrorBadRequest
	ErrorBadRequest(rr, err)

	// Check status respons
	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("Status code tidak sesuai: got %v want %v", status, http.StatusBadRequest)
	}

	// Check the content type
	expectedContentType := "application/json"
	if contentType := rr.Header().Get("Content-Type"); contentType != expectedContentType {
		t.Errorf("Content-Type tidak sesuai: got %v want %v", contentType, expectedContentType)
	}

	// Check respons JSON
	var decodedResponse ErrorResponse
	err = json.NewDecoder(rr.Body).Decode(&decodedResponse)
	if err != nil {
		t.Errorf("Gagal mendekode respons JSON: %v", err)
	}

	if decodedResponse.Code != http.StatusBadRequest {
		t.Errorf("Code tidak sesuai: got %v want %v", decodedResponse.Code, http.StatusBadRequest)
	}
	if decodedResponse.Status != "Bad Request" {
		t.Errorf("Status tidak sesuai: got %v want %v", decodedResponse.Status, "Bad Request")
	}
	if decodedResponse.Error.Message != "Bad Request" {
		t.Errorf("Error message tidak sesuai: got %v want %v", decodedResponse.Error.Message, "Bad Request")
	}
}
