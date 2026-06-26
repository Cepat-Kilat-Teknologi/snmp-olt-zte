package errors

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"syscall"
)

// ErrorType represents the category of error
// Used to distinguish between different types of application errors for proper handling.
type ErrorType string

const (
	ErrorTypeValidation   ErrorType = "VALIDATION_ERROR" // Error type for validation failures
	ErrorTypeNotFound     ErrorType = "NOT_FOUND"        // Error type for resource not found
	ErrorTypeUnauthorized ErrorType = "UNAUTHORIZED"     // Error type for authentication failures (401)
	ErrorTypeSNMP         ErrorType = "SNMP_ERROR"       // Error type for SNMP operations
	ErrorTypeRedis        ErrorType = "REDIS_ERROR"      // Error type for Redis operations
	ErrorTypeConfig       ErrorType = "CONFIG_ERROR"     // Error type for configuration issues
	ErrorTypeInternal     ErrorType = "INTERNAL_ERROR"   // Error type for internal server errors
	// ErrorTypeServiceUnavailable marks a dependency-unreachable condition (the OLT
	// cannot be reached over SNMP). Per the ISP Adapter Standard this maps to HTTP
	// 503, distinct from the service's OWN internal faults which map to HTTP 500.
	ErrorTypeServiceUnavailable ErrorType = "SERVICE_UNAVAILABLE"
)

// AppError represents a structured application error
// containing the type, message, underlying cause, and optional details.
type AppError struct {
	Type    ErrorType      // Category of the error
	Message string         // User-friendly error message
	Err     error          // The underlying error (if any)
	Details map[string]any // Additional context or validation details
}

// Error implements the error interface for AppError
// Returns a formatted error string.
func (e *AppError) Error() string {
	if e.Err != nil { // If there is an underlying error
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Type, e.Message, e.Err) // Include it in the string
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message) // Otherwise just type and message
}

// Unwrap allows errors.Is and errors.As to work with the underlying error
func (e *AppError) Unwrap() error {
	return e.Err
}

// NewValidationError creates a new validation error
// Used when client input fails validation rules.
func NewValidationError(message string, details map[string]any) *AppError {
	return &AppError{
		Type:    ErrorTypeValidation,
		Message: message,
		Details: details,
	}
}

// NewNotFoundError creates a new not-found error
// Used when a requested resource cannot be located.
func NewNotFoundError(resource string, identifier any) *AppError {
	return &AppError{
		Type:    ErrorTypeNotFound,
		Message: fmt.Sprintf("%s not found", resource),
		Details: map[string]any{"identifier": identifier},
	}
}

// NewUnauthorizedError creates a new authentication error (HTTP 401).
// Used when the X-API-Key is missing or not recognized.
func NewUnauthorizedError(message string) *AppError {
	return &AppError{
		Type:    ErrorTypeUnauthorized,
		Message: message,
	}
}

// NewSNMPError creates a new SNMP error
// Used for errors occurring during SNMP communication.
func NewSNMPError(operation string, err error) *AppError {
	return &AppError{
		Type:    ErrorTypeSNMP,
		Message: fmt.Sprintf("SNMP %s failed", operation),
		Err:     err,
	}
}

// deviceUnreachableSubstrings are lowercased markers found in SNMP transport
// errors that indicate the OLT could not be reached over the network — as
// opposed to an internal/programming fault inside this service. gosnmp surfaces
// most transport failures as plain wrapped strings, so a substring scan is the
// reliable fallback when typed checks (net.Error/syscall) do not match.
var deviceUnreachableSubstrings = []string{
	"connection refused",
	"connection reset",
	"connection timed out",
	"i/o timeout",
	"request timeout", // gosnmp: "request timeout (after N retries)"
	"timeout",
	"no response",
	"read udp",
	"write udp",
	"recvfrom",
	"sendto",
	"reading from socket",
	"writing to socket",
	"network is unreachable",
	"host is unreachable",
	"no route to host",
	"broken pipe",
}

// IsDeviceUnreachable reports whether err represents a condition where the OLT
// could not be reached over SNMP (connection refused, i/o timeout, no SNMP
// response, or a socket read/write failure). Such dependency-unreachable
// conditions must surface as HTTP 503 (SERVICE_UNAVAILABLE), distinct from the
// service's own internal faults which remain HTTP 500.
func IsDeviceUnreachable(err error) bool {
	if err == nil {
		return false
	}

	// Typed checks first — robust against message/locale changes.
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}
	if errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.EHOSTUNREACH) ||
		errors.Is(err, syscall.ENETUNREACH) ||
		errors.Is(err, syscall.ETIMEDOUT) ||
		errors.Is(err, syscall.EPIPE) {
		return true
	}

	// String fallback — gosnmp wraps many transport errors as plain strings.
	msg := strings.ToLower(err.Error())
	for _, s := range deviceUnreachableSubstrings {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}

// NewRedisError creates a new Redis error
// Used for errors occurring during Redis operations.
func NewRedisError(operation string, err error) *AppError {
	return &AppError{
		Type:    ErrorTypeRedis,
		Message: fmt.Sprintf("Redis %s failed", operation),
		Err:     err,
	}
}

// NewConfigError creates a new configuration error
// Used for errors related to loading or parsing configuration.
func NewConfigError(message string, err error) *AppError {
	return &AppError{
		Type:    ErrorTypeConfig,
		Message: message,
		Err:     err,
	}
}

// NewInternalError creates a new internal error
// Used for unexpected system errors.
func NewInternalError(message string, err error) *AppError {
	return &AppError{
		Type:    ErrorTypeInternal,
		Message: message,
		Err:     err,
	}
}
