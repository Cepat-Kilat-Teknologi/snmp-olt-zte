package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/reqctx"
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
			expectedBody:   `"error_code":"UNAUTHORIZED"`,
		},
		{
			name:           "missing API key header - 401",
			apiKeyEnv:      "test-secret-key",
			headerKey:      "",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   `"error_code":"UNAUTHORIZED"`,
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

			body := rr.Body.String()
			if tt.expectedStatus == http.StatusUnauthorized {
				// 401 now uses the standard error envelope; assert the error_code
				// is present (request_id varies, so don't match the whole body).
				if !strings.Contains(body, tt.expectedBody) {
					t.Errorf("expected body to contain %q, got %q", tt.expectedBody, body)
				}
			} else if body != tt.expectedBody {
				t.Errorf("expected body %q, got %q", tt.expectedBody, body)
			}
		})
	}
}

func TestAuthenticator_PerUser(t *testing.T) {
	users := map[string]reqctx.Principal{
		"keyA":  {UserID: 1},
		"admin": {Admin: true},
	}
	// Next handler echoes the resolved principal's user_id + admin flag.
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := reqctx.PrincipalFromContext(r.Context())
		if !ok {
			t.Error("expected a principal in context")
		}
		w.WriteHeader(http.StatusOK)
		if p.Admin {
			_, _ = w.Write([]byte("admin"))
		} else {
			_, _ = w.Write([]byte("user"))
		}
	})
	h := Authenticator(users, "")(next)

	tests := []struct {
		name string
		key  string
		want int
		body string
	}{
		{"valid user key", "keyA", http.StatusOK, "user"},
		{"valid admin key", "admin", http.StatusOK, "admin"},
		{"unknown key -> 401", "nope", http.StatusUnauthorized, ""},
		{"missing key -> 401", "", http.StatusUnauthorized, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
			if tt.key != "" {
				req.Header.Set("X-API-Key", tt.key)
			}
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if rr.Code != tt.want {
				t.Fatalf("status=%d, want %d", rr.Code, tt.want)
			}
			if tt.body != "" && rr.Body.String() != tt.body {
				t.Errorf("body=%q, want %q", rr.Body.String(), tt.body)
			}
		})
	}
}

func TestAuthenticator_LegacyKey(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	// Legacy mode (users nil): a non-empty key must match.
	h := Authenticator(nil, "secret")(ok)
	for _, tc := range []struct {
		key  string
		want int
	}{{"secret", http.StatusOK}, {"wrong", http.StatusUnauthorized}, {"", http.StatusUnauthorized}} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
		if tc.key != "" {
			req.Header.Set("X-API-Key", tc.key)
		}
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != tc.want {
			t.Errorf("legacy key %q: status=%d, want %d", tc.key, rr.Code, tc.want)
		}
	}

	// Open mode (users nil, legacyKey ""): everything passes.
	hOpen := Authenticator(nil, "")(ok)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	rr := httptest.NewRecorder()
	hOpen.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("open mode: status=%d, want 200", rr.Code)
	}
}

func TestRequireOLTOwner(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	guard := RequireOLTOwner(1)(ok)

	tests := []struct {
		name string
		ctx  func(r *http.Request) *http.Request
		want int
	}{
		{
			name: "no principal (auth off) -> pass",
			ctx:  func(r *http.Request) *http.Request { return r },
			want: http.StatusOK,
		},
		{
			name: "admin -> pass",
			ctx: func(r *http.Request) *http.Request {
				return r.WithContext(reqctx.WithPrincipal(r.Context(), reqctx.Principal{Admin: true}))
			},
			want: http.StatusOK,
		},
		{
			name: "owner -> pass",
			ctx: func(r *http.Request) *http.Request {
				return r.WithContext(reqctx.WithPrincipal(r.Context(), reqctx.Principal{UserID: 1}))
			},
			want: http.StatusOK,
		},
		{
			name: "other tenant -> 404",
			ctx: func(r *http.Request) *http.Request {
				return r.WithContext(reqctx.WithPrincipal(r.Context(), reqctx.Principal{UserID: 2}))
			},
			want: http.StatusNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.ctx(httptest.NewRequest(http.MethodGet, "/api/v1/olt/x/board/1/pon/1", nil))
			rr := httptest.NewRecorder()
			guard.ServeHTTP(rr, req)
			if rr.Code != tt.want {
				t.Errorf("status=%d, want %d", rr.Code, tt.want)
			}
		})
	}
}
