package errors

import (
	"errors"
	"net"
	"syscall"
	"testing"
)

// netTimeoutErr is a minimal net.Error reporting Timeout()==true.
type netTimeoutErr struct{}

func (netTimeoutErr) Error() string   { return "deadline exceeded" }
func (netTimeoutErr) Timeout() bool   { return true }
func (netTimeoutErr) Temporary() bool { return false }

func TestIsDeviceUnreachable(t *testing.T) {
	unreachable := []struct {
		name string
		err  error
	}{
		{"nil-guard handled separately", errors.New("connection refused")},
		{"connection refused string", errors.New("read udp 10.0.0.2:161->10.0.0.1:161: recvfrom: connection refused")},
		{"i/o timeout string", errors.New("read udp: i/o timeout")},
		{"gosnmp request timeout", errors.New("request timeout (after 3 retries)")},
		{"no route to host", errors.New("dial udp: connect: no route to host")},
		{"network unreachable", errors.New("connect: network is unreachable")},
		{"reading from socket", errors.New("error reading from socket: connection reset by peer")},
		{"typed net timeout", &net.OpError{Op: "read", Net: "udp", Err: netTimeoutErr{}}},
		{"syscall ECONNREFUSED", &net.OpError{Op: "read", Net: "udp", Err: syscall.ECONNREFUSED}},
		{"syscall ETIMEDOUT", syscall.ETIMEDOUT},
	}
	for _, tt := range unreachable {
		t.Run("unreachable/"+tt.name, func(t *testing.T) {
			if !IsDeviceUnreachable(tt.err) {
				t.Errorf("IsDeviceUnreachable(%v) = false, want true", tt.err)
			}
		})
	}

	internal := []struct {
		name string
		err  error
	}{
		{"nil", nil},
		{"no variables in response", errors.New("no variables in response")},
		{"unmarshal", errors.New("failed to unmarshal pdu value")},
		{"generic", errors.New("something internal went wrong")},
	}
	for _, tt := range internal {
		t.Run("internal/"+tt.name, func(t *testing.T) {
			if IsDeviceUnreachable(tt.err) {
				t.Errorf("IsDeviceUnreachable(%v) = true, want false", tt.err)
			}
		})
	}
}

func TestErrorType_Constants(t *testing.T) {
	// Verify all error type constants
	if ErrorTypeValidation != "VALIDATION_ERROR" {
		t.Errorf("Expected ErrorTypeValidation to be 'VALIDATION_ERROR', got '%s'", ErrorTypeValidation)
	}

	if ErrorTypeNotFound != "NOT_FOUND" {
		t.Errorf("Expected ErrorTypeNotFound to be 'NOT_FOUND', got '%s'", ErrorTypeNotFound)
	}

	if ErrorTypeSNMP != "SNMP_ERROR" {
		t.Errorf("Expected ErrorTypeSNMP to be 'SNMP_ERROR', got '%s'", ErrorTypeSNMP)
	}

	if ErrorTypeRedis != "REDIS_ERROR" {
		t.Errorf("Expected ErrorTypeRedis to be 'REDIS_ERROR', got '%s'", ErrorTypeRedis)
	}

	if ErrorTypeConfig != "CONFIG_ERROR" {
		t.Errorf("Expected ErrorTypeConfig to be 'CONFIG_ERROR', got '%s'", ErrorTypeConfig)
	}

	if ErrorTypeInternal != "INTERNAL_ERROR" {
		t.Errorf("Expected ErrorTypeInternal to be 'INTERNAL_ERROR', got '%s'", ErrorTypeInternal)
	}

	if ErrorTypeUnauthorized != "UNAUTHORIZED" {
		t.Errorf("Expected ErrorTypeUnauthorized to be 'UNAUTHORIZED', got '%s'", ErrorTypeUnauthorized)
	}

	if ErrorTypeServiceUnavailable != "SERVICE_UNAVAILABLE" {
		t.Errorf("Expected ErrorTypeServiceUnavailable to be 'SERVICE_UNAVAILABLE', got '%s'", ErrorTypeServiceUnavailable)
	}
}

func TestNewUnauthorizedError(t *testing.T) {
	err := NewUnauthorizedError("invalid or missing API key")
	if err.Type != ErrorTypeUnauthorized {
		t.Errorf("Type = %s, want UNAUTHORIZED", err.Type)
	}
	if err.Message != "invalid or missing API key" {
		t.Errorf("Message = %q", err.Message)
	}
}

func TestAppError_Error_WithUnderlyingError(t *testing.T) {
	underlyingErr := errors.New("connection refused")
	appErr := &AppError{
		Type:    ErrorTypeSNMP,
		Message: "Failed to connect",
		Err:     underlyingErr,
	}

	expected := "SNMP_ERROR: Failed to connect (caused by: connection refused)"
	result := appErr.Error()

	if result != expected {
		t.Errorf("Expected error string '%s', got '%s'", expected, result)
	}
}

func TestAppError_Error_WithoutUnderlyingError(t *testing.T) {
	appErr := &AppError{
		Type:    ErrorTypeValidation,
		Message: "Invalid input",
	}

	expected := "VALIDATION_ERROR: Invalid input"
	result := appErr.Error()

	if result != expected {
		t.Errorf("Expected error string '%s', got '%s'", expected, result)
	}
}

func TestAppError_Unwrap(t *testing.T) {
	underlyingErr := errors.New("original error")
	appErr := &AppError{
		Type: ErrorTypeInternal,
		Err:  underlyingErr,
	}

	unwrapped := appErr.Unwrap()

	if unwrapped != underlyingErr {
		t.Errorf("Expected unwrapped error to be original error")
	}
}

func TestAppError_Unwrap_Nil(t *testing.T) {
	appErr := &AppError{
		Type: ErrorTypeValidation,
		Err:  nil,
	}

	unwrapped := appErr.Unwrap()

	if unwrapped != nil {
		t.Errorf("Expected unwrapped error to be nil, got %v", unwrapped)
	}
}

func TestNewValidationError(t *testing.T) {
	details := map[string]interface{}{
		"field": "email",
		"rule":  "required",
	}

	err := NewValidationError("Email is required", details)

	if err.Type != ErrorTypeValidation {
		t.Errorf("Expected type ErrorTypeValidation, got %s", err.Type)
	}

	if err.Message != "Email is required" {
		t.Errorf("Expected message 'Email is required', got '%s'", err.Message)
	}

	if err.Details == nil {
		t.Error("Expected details to be non-nil")
	}

	if err.Details["field"] != "email" {
		t.Errorf("Expected details field to be 'email', got %v", err.Details["field"])
	}

	if err.Err != nil {
		t.Error("Expected underlying error to be nil")
	}
}

func TestNewNotFoundError(t *testing.T) {
	identifier := map[string]int{"id": 123}
	err := NewNotFoundError("User", identifier)

	if err.Type != ErrorTypeNotFound {
		t.Errorf("Expected type ErrorTypeNotFound, got %s", err.Type)
	}

	expectedMsg := "User not found"
	if err.Message != expectedMsg {
		t.Errorf("Expected message '%s', got '%s'", expectedMsg, err.Message)
	}

	if err.Details == nil {
		t.Error("Expected details to be non-nil")
	}

	if err.Details["identifier"] == nil {
		t.Error("Expected identifier in details")
	}

	if err.Err != nil {
		t.Error("Expected underlying error to be nil")
	}
}

func TestNewNotFoundError_DifferentResources(t *testing.T) {
	tests := []struct {
		resource   string
		identifier interface{}
	}{
		{"ONU", 5},
		{"Configuration", "Board1Pon1"},
		{"Device", 123},
	}

	for _, tt := range tests {
		t.Run(tt.resource, func(t *testing.T) {
			err := NewNotFoundError(tt.resource, tt.identifier)

			expectedMsg := tt.resource + " not found"
			if err.Message != expectedMsg {
				t.Errorf("Expected message '%s', got '%s'", expectedMsg, err.Message)
			}

			if err.Details["identifier"] != tt.identifier {
				t.Errorf("Expected identifier %v, got %v", tt.identifier, err.Details["identifier"])
			}
		})
	}
}

func TestNewNotFoundError_MapIdentifier(t *testing.T) {
	identifier := map[string]string{"serial": "ABC123"}
	err := NewNotFoundError("Device", identifier)

	expectedMsg := "Device not found"
	if err.Message != expectedMsg {
		t.Errorf("Expected message '%s', got '%s'", expectedMsg, err.Message)
	}

	// Can't use == for map comparison, just verify it exists
	if err.Details["identifier"] == nil {
		t.Error("Expected identifier in details")
	}
}

func TestNewSNMPError(t *testing.T) {
	underlyingErr := errors.New("timeout")
	err := NewSNMPError("Get", underlyingErr)

	if err.Type != ErrorTypeSNMP {
		t.Errorf("Expected type ErrorTypeSNMP, got %s", err.Type)
	}

	expectedMsg := "SNMP Get failed"
	if err.Message != expectedMsg {
		t.Errorf("Expected message '%s', got '%s'", expectedMsg, err.Message)
	}

	if err.Err != underlyingErr {
		t.Error("Expected underlying error to match")
	}
}

func TestNewSNMPError_DifferentOperations(t *testing.T) {
	operations := []string{"Get", "Walk", "Set", "Connect"}

	for _, op := range operations {
		t.Run(op, func(t *testing.T) {
			underlyingErr := errors.New("test error")
			err := NewSNMPError(op, underlyingErr)

			expectedMsg := "SNMP " + op + " failed"
			if err.Message != expectedMsg {
				t.Errorf("Expected message '%s', got '%s'", expectedMsg, err.Message)
			}
		})
	}
}

func TestNewRedisError(t *testing.T) {
	underlyingErr := errors.New("connection refused")
	err := NewRedisError("Set", underlyingErr)

	if err.Type != ErrorTypeRedis {
		t.Errorf("Expected type ErrorTypeRedis, got %s", err.Type)
	}

	expectedMsg := "Redis Set failed"
	if err.Message != expectedMsg {
		t.Errorf("Expected message '%s', got '%s'", expectedMsg, err.Message)
	}

	if err.Err != underlyingErr {
		t.Error("Expected underlying error to match")
	}
}

func TestNewRedisError_DifferentOperations(t *testing.T) {
	operations := []string{"Get", "Set", "Delete", "Connect"}

	for _, op := range operations {
		t.Run(op, func(t *testing.T) {
			underlyingErr := errors.New("test error")
			err := NewRedisError(op, underlyingErr)

			expectedMsg := "Redis " + op + " failed"
			if err.Message != expectedMsg {
				t.Errorf("Expected message '%s', got '%s'", expectedMsg, err.Message)
			}
		})
	}
}

func TestNewConfigError(t *testing.T) {
	underlyingErr := errors.New("file not found")
	err := NewConfigError("Failed to load configuration", underlyingErr)

	if err.Type != ErrorTypeConfig {
		t.Errorf("Expected type ErrorTypeConfig, got %s", err.Type)
	}

	if err.Message != "Failed to load configuration" {
		t.Errorf("Expected message 'Failed to load configuration', got '%s'", err.Message)
	}

	if err.Err != underlyingErr {
		t.Error("Expected underlying error to match")
	}
}

func TestNewInternalError(t *testing.T) {
	underlyingErr := errors.New("unexpected panic")
	err := NewInternalError("Internal server error", underlyingErr)

	if err.Type != ErrorTypeInternal {
		t.Errorf("Expected type ErrorTypeInternal, got %s", err.Type)
	}

	if err.Message != "Internal server error" {
		t.Errorf("Expected message 'Internal server error', got '%s'", err.Message)
	}

	if err.Err != underlyingErr {
		t.Error("Expected underlying error to match")
	}
}

func TestAppError_ErrorsIs(t *testing.T) {
	originalErr := errors.New("original error")
	appErr := NewSNMPError("Get", originalErr)

	// Test that errors.Is works with Unwrap
	if !errors.Is(appErr, originalErr) {
		t.Error("Expected errors.Is to find the original error")
	}
}

func TestAppError_ErrorsAs(t *testing.T) {
	appErr := NewValidationError("test", nil)

	// Test that errors.As works
	var target *AppError
	if !errors.As(appErr, &target) {
		t.Error("Expected errors.As to work with AppError")
	}

	if target.Type != ErrorTypeValidation {
		t.Error("Expected to extract the AppError with correct type")
	}
}

func TestAppError_WithDetails(t *testing.T) {
	details := map[string]interface{}{
		"field":      "username",
		"value":      "admin",
		"min_length": 6,
	}

	err := NewValidationError("Username too short", details)

	if len(err.Details) != 3 {
		t.Errorf("Expected 3 details, got %d", len(err.Details))
	}

	if err.Details["field"] != "username" {
		t.Error("Expected field detail to be 'username'")
	}

	if err.Details["min_length"] != 6 {
		t.Error("Expected min_length detail to be 6")
	}
}

func TestAppError_NilDetails(t *testing.T) {
	err := NewValidationError("Test error", nil)

	if err.Details != nil {
		t.Error("Expected Details to be nil when passed nil")
	}
}

func TestAppError_EmptyDetails(t *testing.T) {
	details := make(map[string]interface{})
	err := NewValidationError("Test error", details)

	if err.Details == nil {
		t.Error("Expected Details to be non-nil")
	}

	if len(err.Details) != 0 {
		t.Errorf("Expected empty details, got %d items", len(err.Details))
	}
}

func TestAppError_ComplexDetails(t *testing.T) {
	details := map[string]interface{}{
		"board_id": 1,
		"pon_id":   8,
		"errors": []string{
			"ONU not found",
			"Invalid configuration",
		},
		"metadata": map[string]string{
			"source": "SNMP",
			"target": "192.168.1.1",
		},
	}

	err := NewValidationError("Complex validation failed", details)

	if err.Details == nil {
		t.Error("Expected Details to be non-nil")
	}

	errorsList, ok := err.Details["errors"].([]string)
	if !ok || len(errorsList) != 2 {
		t.Error("Expected errors list with 2 items")
	}

	metadata, ok := err.Details["metadata"].(map[string]string)
	if !ok || metadata["source"] != "SNMP" {
		t.Error("Expected metadata with source 'SNMP'")
	}
}
