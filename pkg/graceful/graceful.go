package graceful

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Shutdown gracefully shuts down the HTTP server when the context is done or an OS signal is received.
// Supports both HTTP and HTTPS based on environment configuration.
// Set USE_TLS=true and provide TLS_CERT_FILE and TLS_KEY_FILE for HTTPS.
func Shutdown(ctx context.Context, server *http.Server) error {
	ch := make(chan error, 1) // Create a buffered channel to capture server errors

	// Check if TLS/HTTPS should be used (from environment variable)
	useTLS := os.Getenv("USE_TLS") == "true"
	certFile := os.Getenv("TLS_CERT_FILE")
	keyFile := os.Getenv("TLS_KEY_FILE")

	go func() { // Start a server in a goroutine
		var err error

		if useTLS {
			// Validate TLS configuration
			if certFile == "" || keyFile == "" {
				ch <- fmt.Errorf("TLS enabled but TLS_CERT_FILE or TLS_KEY_FILE not provided")
				close(ch)
				return
			}
			log.Printf("Starting HTTPS server on %s with TLS", server.Addr)
			err = server.ListenAndServeTLS(certFile, keyFile) // Start HTTPS server
		} else {
			log.Printf("Starting HTTP server on %s", server.Addr)
			err = server.ListenAndServe() // Start HTTP server
		}

		if err != nil && !errors.Is(err, http.ErrServerClosed) { // Check for unexpected errors
			ch <- fmt.Errorf("failed to start server: %v", err) // Send error to channel
		}
		close(ch) // Close channel when the server stops
	}()

	// Create a channel to capture OS signals (e.g., SIGINT or SIGTERM).
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM) // Register a channel to receive interrupt and term signals

	select { // Wait for one of the cases to happen
	case err := <-ch: // If server error occurred
		return err // Return the error
	case <-ctx.Done(): // If context is canceled (e.g., external shutdown request)
		timeoutCtx, cancel := context.WithTimeout(context.Background(), time.Second*10) // Create a shutdown context with 10s timeout
		defer cancel()                                                                  // Ensure cancellation
		_ = server.Shutdown(timeoutCtx)                                                 // Attempt a graceful shutdown (error is non-critical during shutdown)
	case sig := <-signalCh: // If OS signal received
		log.Printf("Received signal: %v. Shutting down gracefully...", sig) // Log signal reception

		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second*10) // Create a shutdown context with 10s timeout
		defer cancel()                                                                   // Ensure cancellation
		_ = server.Shutdown(shutdownCtx)                                                 // Attempt a graceful shutdown (error is non-critical during shutdown)
	}

	return nil // Return nil indicating a successful (or handled) shutdown
}
