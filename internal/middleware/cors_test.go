package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCorsMiddleware_DefaultConfig(t *testing.T) {
	// Clear environment variables
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	t.Setenv("CORS_ALLOWED_METHODS", "")
	t.Setenv("CORS_ALLOWED_HEADERS", "")
	t.Setenv("CORS_ALLOW_CREDENTIALS", "")
	t.Setenv("CORS_MAX_AGE", "")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	middleware := CorsMiddleware()
	wrappedHandler := middleware(handler)

	// Test CORS preflight request
	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")

	rr := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rr, req)

	// Should allow CORS
	if rr.Code != http.StatusOK && rr.Code != http.StatusNoContent {
		t.Errorf("Expected status OK or NoContent for OPTIONS, got %d", rr.Code)
	}
}

func TestCorsMiddleware_CustomOrigins(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://example.com,https://test.com")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := CorsMiddleware()
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://example.com")

	rr := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %d", rr.Code)
	}
}

func TestCorsMiddleware_CustomMethods(t *testing.T) {
	t.Setenv("CORS_ALLOWED_METHODS", "GET,POST,PUT")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := CorsMiddleware()
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")

	rr := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK && rr.Code != http.StatusNoContent {
		t.Errorf("Expected status OK or NoContent, got %d", rr.Code)
	}
}

func TestCorsMiddleware_CustomHeaders(t *testing.T) {
	t.Setenv("CORS_ALLOWED_HEADERS", "Authorization,Content-Type,X-Custom-Header")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := CorsMiddleware()
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://example.com")

	rr := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %d", rr.Code)
	}
}

func TestCorsMiddleware_AllowCredentials(t *testing.T) {
	t.Setenv("CORS_ALLOW_CREDENTIALS", "true")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := CorsMiddleware()
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://example.com")

	rr := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %d", rr.Code)
	}
}

func TestCorsMiddleware_MaxAge(t *testing.T) {
	t.Setenv("CORS_MAX_AGE", "600")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := CorsMiddleware()
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "https://example.com")

	rr := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK && rr.Code != http.StatusNoContent {
		t.Errorf("Expected status OK or NoContent, got %d", rr.Code)
	}
}

func TestGetEnvAsSlice(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		envValue     string
		setEnv       bool
		defaultValue []string
		expected     []string
	}{
		{
			name:         "Valid comma-separated values",
			key:          "TEST_SLICE",
			envValue:     "value1,value2,value3",
			setEnv:       true,
			defaultValue: []string{"default"},
			expected:     []string{"value1", "value2", "value3"},
		},
		{
			name:         "Empty environment variable - use default",
			key:          "TEST_SLICE",
			envValue:     "",
			setEnv:       false,
			defaultValue: []string{"default1", "default2"},
			expected:     []string{"default1", "default2"},
		},
		{
			name:         "Values with spaces - should trim",
			key:          "TEST_SLICE",
			envValue:     " value1 , value2 , value3 ",
			setEnv:       true,
			defaultValue: []string{"default"},
			expected:     []string{"value1", "value2", "value3"},
		},
		{
			name:         "Single value",
			key:          "TEST_SLICE",
			envValue:     "single",
			setEnv:       true,
			defaultValue: []string{"default"},
			expected:     []string{"single"},
		},
		{
			name:         "Values with empty entries - should skip",
			key:          "TEST_SLICE",
			envValue:     "value1,,value2,  ,value3",
			setEnv:       true,
			defaultValue: []string{"default"},
			expected:     []string{"value1", "value2", "value3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv {
				t.Setenv(tt.key, tt.envValue)
			} else {
				t.Setenv(tt.key, "")
			}

			result := getEnvAsSlice(tt.key, tt.defaultValue)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected length %d, got %d", len(tt.expected), len(result))
			}

			for i, val := range result {
				if val != tt.expected[i] {
					t.Errorf("At index %d: expected %s, got %s", i, tt.expected[i], val)
				}
			}
		})
	}
}

func TestGetEnvAsBool(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		envValue     string
		setEnv       bool
		defaultValue bool
		expected     bool
	}{
		{
			name:         "True value",
			key:          "TEST_BOOL",
			envValue:     "true",
			setEnv:       true,
			defaultValue: false,
			expected:     true,
		},
		{
			name:         "False value",
			key:          "TEST_BOOL",
			envValue:     "false",
			setEnv:       true,
			defaultValue: true,
			expected:     false,
		},
		{
			name:         "1 value - parsed as true",
			key:          "TEST_BOOL",
			envValue:     "1",
			setEnv:       true,
			defaultValue: false,
			expected:     true,
		},
		{
			name:         "0 value - parsed as false",
			key:          "TEST_BOOL",
			envValue:     "0",
			setEnv:       true,
			defaultValue: true,
			expected:     false,
		},
		{
			name:         "Empty value - use default",
			key:          "TEST_BOOL",
			envValue:     "",
			setEnv:       false,
			defaultValue: true,
			expected:     true,
		},
		{
			name:         "Invalid value - use default",
			key:          "TEST_BOOL",
			envValue:     "invalid",
			setEnv:       true,
			defaultValue: false,
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv {
				t.Setenv(tt.key, tt.envValue)
			} else {
				t.Setenv(tt.key, "")
			}

			result := getEnvAsBool(tt.key, tt.defaultValue)

			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetEnvAsInt(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		envValue     string
		setEnv       bool
		defaultValue int
		expected     int
	}{
		{
			name:         "Valid integer",
			key:          "TEST_INT",
			envValue:     "42",
			setEnv:       true,
			defaultValue: 10,
			expected:     42,
		},
		{
			name:         "Empty value - use default",
			key:          "TEST_INT",
			envValue:     "",
			setEnv:       false,
			defaultValue: 10,
			expected:     10,
		},
		{
			name:         "Invalid value - use default",
			key:          "TEST_INT",
			envValue:     "invalid",
			setEnv:       true,
			defaultValue: 5,
			expected:     5,
		},
		{
			name:         "Negative integer",
			key:          "TEST_INT",
			envValue:     "-10",
			setEnv:       true,
			defaultValue: 0,
			expected:     -10,
		},
		{
			name:         "Zero",
			key:          "TEST_INT",
			envValue:     "0",
			setEnv:       true,
			defaultValue: 10,
			expected:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv {
				t.Setenv(tt.key, tt.envValue)
			} else {
				t.Setenv(tt.key, "")
			}

			result := getEnvAsInt(tt.key, tt.defaultValue)

			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestCorsMiddleware_ActualRequest(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "")

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	})

	middleware := CorsMiddleware()
	handler := middleware(testHandler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Origin", "https://example.com")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %d", rr.Code)
	}

	if body := strings.TrimSpace(rr.Body.String()); body != "success" {
		t.Errorf("Expected body 'success', got '%s'", body)
	}
}
