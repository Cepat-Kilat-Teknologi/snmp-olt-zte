package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func noopHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
}

func TestDefaultAPIVersionConfig(t *testing.T) {
	cfg := DefaultAPIVersionConfig("v1", "3.0.0", "abcd123")
	if cfg.APIVersion != "v1" || cfg.AppVersion != "3.0.0" || cfg.Commit != "abcd123" {
		t.Errorf("unexpected config values: %+v", cfg)
	}
	if cfg.APIVersionHeader != "X-API-Version" {
		t.Errorf("default APIVersionHeader = %q", cfg.APIVersionHeader)
	}
	if cfg.AppVersionHeader != "X-App-Version" {
		t.Errorf("default AppVersionHeader = %q", cfg.AppVersionHeader)
	}
	if cfg.CommitHeader != "X-Build-Commit" {
		t.Errorf("default CommitHeader = %q", cfg.CommitHeader)
	}
}

func TestAPIVersionHeader_SetsAllConfiguredHeaders(t *testing.T) {
	cfg := DefaultAPIVersionConfig("v1", "3.0.0", "abcd123")
	h := APIVersionHeader(cfg)(noopHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if got := rr.Header().Get("X-API-Version"); got != "v1" {
		t.Errorf("X-API-Version = %q want v1", got)
	}
	if got := rr.Header().Get("X-App-Version"); got != "3.0.0" {
		t.Errorf("X-App-Version = %q want 3.0.0", got)
	}
	if got := rr.Header().Get("X-Build-Commit"); got != "abcd123" {
		t.Errorf("X-Build-Commit = %q want abcd123", got)
	}
}

func TestAPIVersionHeader_OmitsEmptyFields(t *testing.T) {
	// Empty config values should be skipped, not written as empty headers.
	cfg := APIVersionConfig{}
	h := APIVersionHeader(cfg)(noopHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	for _, name := range []string{"X-API-Version", "X-App-Version", "X-Build-Commit"} {
		if got := rr.Header().Get(name); got != "" {
			t.Errorf("%s should be unset when config is empty, got %q", name, got)
		}
	}
}

func TestAPIVersionHeader_UsesCustomHeaderNames(t *testing.T) {
	cfg := APIVersionConfig{
		APIVersion:       "v2",
		APIVersionHeader: "X-Custom-API",
		AppVersion:       "1.2.3",
		AppVersionHeader: "X-Custom-App",
		Commit:           "deadbeef",
		CommitHeader:     "X-Custom-Commit",
	}
	h := APIVersionHeader(cfg)(noopHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Header().Get("X-Custom-API") != "v2" {
		t.Errorf("X-Custom-API not set correctly")
	}
	if rr.Header().Get("X-Custom-App") != "1.2.3" {
		t.Errorf("X-Custom-App not set correctly")
	}
	if rr.Header().Get("X-Custom-Commit") != "deadbeef" {
		t.Errorf("X-Custom-Commit not set correctly")
	}
}

func TestAPIVersionHeader_DefaultHeaderNamesWhenMissing(t *testing.T) {
	// When the config has values but no header names, defaults fill in.
	cfg := APIVersionConfig{
		APIVersion: "v1",
		AppVersion: "3.0.0",
		Commit:     "abcd123",
	}
	h := APIVersionHeader(cfg)(noopHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Header().Get("X-API-Version") == "" ||
		rr.Header().Get("X-App-Version") == "" ||
		rr.Header().Get("X-Build-Commit") == "" {
		t.Error("expected default headers to be filled when names omitted from config")
	}
}
