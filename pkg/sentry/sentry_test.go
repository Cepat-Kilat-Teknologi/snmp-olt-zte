package sentry

import (
	"errors"
	"testing"
)

func TestInit_EmptyDSN_ReturnsNil(t *testing.T) {
	if err := Init("", "test", "v1.0.0"); err != nil {
		t.Fatalf("expected nil for empty DSN, got %v", err)
	}
}

func TestInit_InvalidDSN_ReturnsError(t *testing.T) {
	err := Init("not-a-valid-dsn", "test", "v1.0.0")
	if err == nil {
		t.Fatal("expected error for invalid DSN, got nil")
	}
}

func TestCaptureException_NoPanic(t *testing.T) {
	// Should not panic even when Sentry is not initialized.
	CaptureException(errors.New("test error"))
}

func TestCaptureMessage_NoPanic(t *testing.T) {
	// Should not panic even when Sentry is not initialized.
	CaptureMessage("test message")
}

func TestFlush_NoPanic(t *testing.T) {
	// Should not panic even when Sentry is not initialized.
	Flush()
}
