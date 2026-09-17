// Package main provides a minimal healthcheck binary for use in distroless
// Docker HEALTHCHECK instructions. It performs an HTTP GET to the local
// /healthz endpoint and exits 0 on success, 1 on failure.
package main

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

func main() {
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8081"
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%s/healthz", port))
	if err != nil {
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		os.Exit(1)
	}
}
