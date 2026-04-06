package utils

import (
	"encoding/json"
	"errors"
	"net/http"

	apperrors "github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/errors" // Import custom errors
	"github.com/rs/zerolog/log"                                                       // Import logger
)

// SendJSONResponse is a helper function to send a JSON response
// Writes the appropriate headers, status code, and serializes the data to the response body.
func SendJSONResponse(w http.ResponseWriter, statusCode int, response interface{}) {
	w.Header().Set("Content-Type", "application/json") // Set the content type
	w.WriteHeader(statusCode)                          // Set the status code
	err := json.NewEncoder(w).Encode(response)         // Encode and write JSON
	if err != nil {
		return // Silently return if writing fails (logger could be added here if needed)
	}
}

// HandleError converts AppError to appropriate HTTP response
// Maps custom application error types to standard HTTP status codes.
// Logs errors at appropriate levels for Prometheus/Grafana/Loki monitoring.
func HandleError(w http.ResponseWriter, err error) {
	var appErr *apperrors.AppError

	// Check if it's our custom error
	if errors.As(err, &appErr) {
		switch appErr.Type {
		case apperrors.ErrorTypeValidation: // Validation error -> 400 Bad Request
			// Log as WARN - client sent invalid data (not critical, expected behavior)
			log.Warn().
				Str("error_type", string(appErr.Type)).
				Str("message", appErr.Message).
				Interface("details", appErr.Details).
				Msg("Validation error")
			ErrorBadRequest(w, appErr)

		case apperrors.ErrorTypeNotFound: // Not Found error -> 404 Not Found
			// Log as DEBUG - resource not found (normal operation, not an error)
			log.Debug().
				Str("error_type", string(appErr.Type)).
				Str("message", appErr.Message).
				Interface("details", appErr.Details).
				Msg("Resource not found")
			ErrorNotFound(w, appErr)

		case apperrors.ErrorTypeSNMP, apperrors.ErrorTypeRedis, apperrors.ErrorTypeInternal: // Systems errors -> 500 Internal Error
			// Log as ERROR - real system error (already logged upstream, but log here for completeness)
			log.Error().
				Str("error_type", string(appErr.Type)).
				Str("message", appErr.Message).
				Err(appErr.Err).
				Interface("details", appErr.Details).
				Msg("Internal error")
			ErrorInternalServerError(w, appErr)

		case apperrors.ErrorTypeConfig: // Config error -> 500 Internal Error
			// Log as ERROR - configuration error (critical)
			log.Error().
				Str("error_type", string(appErr.Type)).
				Str("message", appErr.Message).
				Err(appErr.Err).
				Msg("Configuration error")
			ErrorInternalServerError(w, appErr)

		default: // Unknown error type -> 500 Internal Error
			// Log as ERROR - unknown error type (critical)
			log.Error().
				Str("error_type", string(appErr.Type)).
				Str("message", appErr.Message).
				Msg("Unknown error type")
			ErrorInternalServerError(w, appErr)
		}
		return
	}

	// Fallback for non-AppError errors
	log.Error().
		Err(err).
		Msg("Unhandled error")
	ErrorInternalServerError(w, err)
}

// ErrorBadRequest is a helper function to send a 400 Bad Request response
func ErrorBadRequest(w http.ResponseWriter, err error) {
	var appErr *apperrors.AppError
	detail := ErrorDetail{Message: err.Error()}
	if errors.As(err, &appErr) {
		detail.Type = string(appErr.Type)
		detail.Message = appErr.Message
		detail.Details = appErr.Details
	}
	resp := ErrorResponse{
		Code:   http.StatusBadRequest,
		Status: "Bad Request",
		Error:  detail,
	}
	SendJSONResponse(w, http.StatusBadRequest, resp)
}

// ErrorInternalServerError is a helper function to send a 500 Internal Server Error response
func ErrorInternalServerError(w http.ResponseWriter, err error) {
	var appErr *apperrors.AppError
	detail := ErrorDetail{Message: err.Error()}
	if errors.As(err, &appErr) {
		detail.Type = string(appErr.Type)
		detail.Message = appErr.Message
		detail.Details = appErr.Details
	}
	resp := ErrorResponse{
		Code:   http.StatusInternalServerError,
		Status: "Internal Server Error",
		Error:  detail,
	}
	SendJSONResponse(w, http.StatusInternalServerError, resp)
}

// ErrorNotFound is a helper function to send a 404 Not Found response
func ErrorNotFound(w http.ResponseWriter, err error) {
	var appErr *apperrors.AppError
	detail := ErrorDetail{Message: err.Error()}
	if errors.As(err, &appErr) {
		detail.Type = string(appErr.Type)
		detail.Message = appErr.Message
		detail.Details = appErr.Details
	}
	resp := ErrorResponse{
		Code:   http.StatusNotFound,
		Status: "Not Found",
		Error:  detail,
	}
	SendJSONResponse(w, http.StatusNotFound, resp)
}
