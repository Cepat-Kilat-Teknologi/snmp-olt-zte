package middleware

import (
	"net/http"
	"os"
)

// APIKeyAuth validates the X-API-Key header against the API_KEY environment variable.
// If API_KEY is not set, authentication is disabled.
func APIKeyAuth(next http.Handler) http.Handler {
	apiKey := os.Getenv("API_KEY")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if apiKey == "" {
			next.ServeHTTP(w, r)
			return
		}

		key := r.Header.Get("X-API-Key")
		if key != apiKey {
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, `{"code":401,"status":"Unauthorized","message":"invalid or missing API key"}`, http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
