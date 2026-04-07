package graceful

import (
	"context"
	"errors"
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestShutdown_ContextCancellation(t *testing.T) {
	// Create a test server
	mux := http.NewServeMux()
	mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server := &http.Server{
		Addr:    ":0", // Use random available port
		Handler: mux,
	}

	// Create a context with cancellation
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Run Shutdown in a goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- Shutdown(ctx, server)
	}()

	// Wait a bit for the server to start
	time.Sleep(50 * time.Millisecond)

	// Cancel the context
	cancel()

	// Wait for shutdown to complete
	select {
	case err := <-errChan:
		if err != nil {
			t.Errorf("Expected nil error on context cancellation, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Shutdown did not complete in time")
	}
}

func TestShutdown_InvalidTLSConfig(t *testing.T) {
	// Set TLS enabled but without cert/key files
	t.Setenv("USE_TLS", "true")
	t.Setenv("TLS_CERT_FILE", "")
	t.Setenv("TLS_KEY_FILE", "")

	mux := http.NewServeMux()
	server := &http.Server{
		Addr:    ":0",
		Handler: mux,
	}

	ctx := context.Background()

	err := Shutdown(ctx, server)

	if err == nil {
		t.Error("Expected error for missing TLS cert/key files")
	}

	expectedMsg := "TLS enabled but TLS_CERT_FILE or TLS_KEY_FILE not provided"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestShutdown_HTTPMode(t *testing.T) {
	// Ensure TLS is disabled
	t.Setenv("USE_TLS", "")

	mux := http.NewServeMux()
	mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server := &http.Server{
		Addr:    "127.0.0.1:0", // Bind to localhost with random port
		Handler: mux,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- Shutdown(ctx, server)
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Shutdown via context cancellation
	cancel()

	select {
	case err := <-errChan:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Errorf("Expected nil or ErrServerClosed, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Shutdown did not complete in time")
	}
}

func TestShutdown_SignalHandling(t *testing.T) {
	// Skip this test in short mode as it involves signals
	if testing.Short() {
		t.Skip("Skipping signal handling test in short mode")
	}

	t.Setenv("USE_TLS", "")

	mux := http.NewServeMux()
	server := &http.Server{
		Addr:    "127.0.0.1:0",
		Handler: mux,
	}

	ctx := context.Background()

	errChan := make(chan error, 1)
	go func() {
		errChan <- Shutdown(ctx, server)
	}()

	// Give server time to start and register signal handlers
	time.Sleep(100 * time.Millisecond)

	// Send interrupt signal to self
	proc, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("Failed to find own process: %v", err)
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("Failed to send SIGTERM: %v", err)
	}

	// Wait for shutdown to complete
	select {
	case err := <-errChan:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Errorf("Expected nil or ErrServerClosed after signal, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("Shutdown did not complete after signal")
	}
}

func TestShutdown_TLSConfigPresent(t *testing.T) {
	// This test would require actual cert/key files
	// For now, we just verify the configuration is read correctly
	t.Setenv("USE_TLS", "true")
	t.Setenv("TLS_CERT_FILE", "/path/to/cert.pem")
	t.Setenv("TLS_KEY_FILE", "/path/to/key.pem")

	mux := http.NewServeMux()
	server := &http.Server{
		Addr:    ":0",
		Handler: mux,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- Shutdown(ctx, server)
	}()

	// Wait a bit
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Should get error because cert/key files don't exist
	select {
	case err := <-errChan:
		if err == nil {
			t.Error("Expected error for non-existent cert/key files")
		}
		// Error is expected (file not found)
	case <-time.After(2 * time.Second):
		t.Error("Shutdown did not complete in time")
	}
}

func TestShutdown_ServerStartFailure(t *testing.T) {
	t.Setenv("USE_TLS", "")

	mux := http.NewServeMux()

	// Create a server with an invalid address to force startup failure
	server := &http.Server{
		Addr:    "invalid:address:12345", // Invalid address format
		Handler: mux,
	}

	ctx := context.Background()

	err := Shutdown(ctx, server)

	// Should get an error from server startup
	if err == nil {
		t.Error("Expected error for invalid server address")
	}
}

func TestShutdown_MultipleShutdownMethods(t *testing.T) {
	// Test that different shutdown triggers work
	tests := []struct {
		name       string
		useTLS     string
		shouldFail bool
	}{
		{
			name:       "HTTP mode",
			useTLS:     "false",
			shouldFail: false,
		},
		{
			name:       "HTTPS mode without certs",
			useTLS:     "true",
			shouldFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("USE_TLS", tt.useTLS)

			mux := http.NewServeMux()
			server := &http.Server{
				Addr:    ":0",
				Handler: mux,
			}

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			errChan := make(chan error, 1)
			go func() {
				errChan <- Shutdown(ctx, server)
			}()

			time.Sleep(50 * time.Millisecond)
			cancel()

			select {
			case err := <-errChan:
				if tt.shouldFail && err == nil {
					t.Error("Expected error but got nil")
				}
				if !tt.shouldFail && err != nil && !errors.Is(err, http.ErrServerClosed) {
					t.Errorf("Expected nil or ErrServerClosed, got %v", err)
				}
			case <-time.After(2 * time.Second):
				t.Error("Test timed out")
			}
		})
	}
}
