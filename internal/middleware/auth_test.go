package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIKeyAuth(t *testing.T) {
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	tests := []struct {
		name           string
		apiKeyEnv      string
		headerKey      string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "no API_KEY env set - request passes through",
			apiKeyEnv:      "",
			headerKey:      "",
			expectedStatus: http.StatusOK,
			expectedBody:   "ok",
		},
		{
			name:           "valid API key - request passes through",
			apiKeyEnv:      "test-secret-key",
			headerKey:      "test-secret-key",
			expectedStatus: http.StatusOK,
			expectedBody:   "ok",
		},
		{
			name:           "invalid API key - 401",
			apiKeyEnv:      "test-secret-key",
			headerKey:      "wrong-key",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   `{"code":401,"status":"Unauthorized","message":"invalid or missing API key"}`,
		},
		{
			name:           "missing API key header - 401",
			apiKeyEnv:      "test-secret-key",
			headerKey:      "",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   `{"code":401,"status":"Unauthorized","message":"invalid or missing API key"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("API_KEY", tt.apiKeyEnv)

			handler := APIKeyAuth(okHandler)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
			if tt.headerKey != "" {
				req.Header.Set("X-API-Key", tt.headerKey)
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			// Trim trailing newline from http.Error
			body := rr.Body.String()
			if tt.expectedStatus == http.StatusUnauthorized {
				// http.Error appends a newline
				body = body[:len(body)-1]
			}
			if body != tt.expectedBody {
				t.Errorf("expected body %q, got %q", tt.expectedBody, body)
			}
		})
	}
}
