package errors

import (
	"fmt"
)

// ErrorType represents the category of error
// Used to distinguish between different types of application errors for proper handling.
type ErrorType string

const (
	ErrorTypeValidation ErrorType = "VALIDATION_ERROR" // Error type for validation failures
	ErrorTypeNotFound   ErrorType = "NOT_FOUND"        // Error type for resource not found
	ErrorTypeSNMP       ErrorType = "SNMP_ERROR"       // Error type for SNMP operations
	ErrorTypeRedis      ErrorType = "REDIS_ERROR"      // Error type for Redis operations
	ErrorTypeConfig     ErrorType = "CONFIG_ERROR"     // Error type for configuration issues
	ErrorTypeInternal   ErrorType = "INTERNAL_ERROR"   // Error type for internal server errors
)

// AppError represents a structured application error
// containing the type, message, underlying cause, and optional details.
type AppError struct {
	Type    ErrorType              // Category of the error
	Message string                 // User-friendly error message
	Err     error                  // The underlying error (if any)
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

// NewSNMPError creates a new SNMP error
// Used for errors occurring during SNMP communication.
func NewSNMPError(operation string, err error) *AppError {
	return &AppError{
		Type:    ErrorTypeSNMP,
		Message: fmt.Sprintf("SNMP %s failed", operation),
		Err:     err,
	}
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
