package utils

import (
	"encoding/json"
	"errors"
	"net/http"

	apperrors "github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/errors"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/pkg/logger"
	"go.uber.org/zap"
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

// requestIDFromRequest extracts the request ID from the HTTP request context.
// Returns empty string if not present.
func requestIDFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	return RequestIDFromContext(r.Context())
}

// buildErrorResponse constructs an ErrorResponse from an error, extracting the
// error code and data payload from AppError when possible.
func buildErrorResponse(code int, status string, requestID string, err error) ErrorResponse {
	resp := ErrorResponse{
		Code:      code,
		Status:    status,
		ErrorCode: string(apperrors.ErrorTypeInternal),
		RequestID: requestID,
	}

	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		resp.ErrorCode = string(appErr.Type)
		if len(appErr.Details) > 0 {
			resp.Data = map[string]any{
				"message": appErr.Message,
				"details": appErr.Details,
			}
		} else {
			resp.Data = appErr.Message
		}
		return resp
	}

	// Fallback for non-AppError errors
	if err != nil {
		resp.Data = err.Error()
	}
	return resp
}

// HandleError converts AppError to appropriate HTTP response.
// Maps custom application error types to standard HTTP status codes.
// Logs errors at appropriate levels for Prometheus/Grafana/Loki monitoring.
func HandleError(w http.ResponseWriter, r *http.Request, err error) {
	var appErr *apperrors.AppError
	requestID := requestIDFromRequest(r)

	// Check if it's our custom error
	if errors.As(err, &appErr) {
		baseFields := []zap.Field{
			zap.String("error_code", string(appErr.Type)),
			zap.String("request_id", requestID),
			zap.String("message", appErr.Message),
		}
		if len(appErr.Details) > 0 {
			baseFields = append(baseFields, zap.Any("details", appErr.Details))
		}

		switch appErr.Type {
		case apperrors.ErrorTypeValidation: // -> 400 Bad Request
			logger.Warn("validation_error", baseFields...)
			writeError(w, http.StatusBadRequest, "Bad Request", requestID, appErr)

		case apperrors.ErrorTypeNotFound: // -> 404 Not Found
			logger.Debug("resource_not_found", baseFields...)
			writeError(w, http.StatusNotFound, "Not Found", requestID, appErr)

		case apperrors.ErrorTypeUnauthorized: // -> 401 Unauthorized
			logger.Warn("unauthorized", baseFields...)
			writeError(w, http.StatusUnauthorized, "Unauthorized", requestID, appErr)

		case apperrors.ErrorTypeSNMP, apperrors.ErrorTypeRedis, apperrors.ErrorTypeInternal: // -> 500
			fields := append(baseFields, zap.Error(appErr.Err))
			logger.Error("internal_error", fields...)
			writeError(w, http.StatusInternalServerError, "Internal Server Error", requestID, appErr)

		case apperrors.ErrorTypeConfig: // -> 500
			fields := append(baseFields, zap.Error(appErr.Err))
			logger.Error("configuration_error", fields...)
			writeError(w, http.StatusInternalServerError, "Internal Server Error", requestID, appErr)

		default: // -> 500
			logger.Error("unknown_error_type", baseFields...)
			writeError(w, http.StatusInternalServerError, "Internal Server Error", requestID, appErr)
		}
		return
	}

	// Fallback for non-AppError errors
	logger.Error("unhandled_error",
		zap.String("request_id", requestID),
		zap.Error(err),
	)
	writeError(w, http.StatusInternalServerError, "Internal Server Error", requestID, err)
}

func writeError(w http.ResponseWriter, code int, status, requestID string, err error) {
	resp := buildErrorResponse(code, status, requestID, err)
	SendJSONResponse(w, code, resp)
}

// ErrorBadRequest is a helper function to send a 400 Bad Request response.
func ErrorBadRequest(w http.ResponseWriter, r *http.Request, err error) {
	writeError(w, http.StatusBadRequest, "Bad Request", requestIDFromRequest(r), err)
}

// ErrorInternalServerError is a helper function to send a 500 Internal Server Error response.
func ErrorInternalServerError(w http.ResponseWriter, r *http.Request, err error) {
	writeError(w, http.StatusInternalServerError, "Internal Server Error", requestIDFromRequest(r), err)
}

// ErrorNotFound is a helper function to send a 404 Not Found response.
func ErrorNotFound(w http.ResponseWriter, r *http.Request, err error) {
	writeError(w, http.StatusNotFound, "Not Found", requestIDFromRequest(r), err)
}

// ErrorUnauthorized is a helper function to send a 401 Unauthorized response.
func ErrorUnauthorized(w http.ResponseWriter, r *http.Request, err error) {
	writeError(w, http.StatusUnauthorized, "Unauthorized", requestIDFromRequest(r), err)
}
