package config

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetchRegistryOLTS(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/registry/snmp" || r.Header.Get("X-API-Key") != "k" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"code":200,"status":"success","data":[
			{"id":"c320-01","user_id":1,"host":"10.0.0.1","port":161,"community":"pub","boards":"1,2"}
		]}`))
	}))
	defer srv.Close()

	js, err := fetchRegistryOLTS(srv.URL, "k")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	// The returned string must be a JSON array that buildOLTRegistry can parse.
	if !strings.HasPrefix(strings.TrimSpace(js), "[") || !strings.Contains(js, "c320-01") {
		t.Fatalf("unexpected payload: %s", js)
	}
	olts, def, err := buildOLTRegistry(js, "", OLTRuntimeConfig{})
	if err != nil {
		t.Fatalf("buildOLTRegistry on fetched json: %v", err)
	}
	if len(olts) != 1 || olts[0].ID != "c320-01" || def != "c320-01" {
		t.Fatalf("registry from fetch wrong: %+v default=%s", olts, def)
	}
}

func TestFetchRegistryOLTS_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"code":200,"status":"success","data":[]}`))
	}))
	defer srv.Close()
	js, err := fetchRegistryOLTS(srv.URL, "")
	if err != nil || js != "" {
		t.Fatalf("empty inventory should yield \"\": js=%q err=%v", js, err)
	}
}

func TestFetchRegistryOLTS_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	if _, err := fetchRegistryOLTS(srv.URL, ""); err == nil {
		t.Fatal("expected error on non-200")
	}
}

// The cold-start race: device-registry is unreachable for the first few attempts,
// then comes up. The retry wrapper must absorb that and return the inventory
// rather than failing — the whole point of the v0.5.1 fix.
func TestFetchRegistryOLTSWithRetry_RecoversAfterRegistryReady(t *testing.T) {
	var attempts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts < 3 { // first two calls = "not ready yet"
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`{"code":200,"status":"success","data":[
			{"id":"c320-01","user_id":1,"host":"10.0.0.1","port":161,"community":"pub","boards":"1,2"}
		]}`))
	}))
	defer srv.Close()

	// Don't actually sleep; just count the backoffs.
	var slept int
	restoreSleep := registrySleep
	registrySleep = func(time.Duration) { slept++ }
	defer func() { registrySleep = restoreSleep }()

	js, err := fetchRegistryOLTSWithRetry(srv.URL, "")
	if err != nil {
		t.Fatalf("expected recovery after registry came up, got: %v", err)
	}
	if !strings.Contains(js, "c320-01") {
		t.Fatalf("unexpected payload after retry: %s", js)
	}
	if attempts != 3 || slept != 2 {
		t.Fatalf("expected 3 attempts / 2 backoffs, got attempts=%d slept=%d", attempts, slept)
	}
}

// A registry that never recovers must fail fast once the startup window is
// exhausted — we must never silently boot with wrong/empty config.
func TestFetchRegistryOLTSWithRetry_GivesUpAfterWindow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	// Tiny window + a fake clock that advances on each sleep so the loop ends
	// deterministically without wall-clock delay.
	t.Setenv("REGISTRY_STARTUP_TIMEOUT", "5s")
	now := time.Unix(0, 0)
	restoreNow, restoreSleep := registryNow, registrySleep
	registryNow = func() time.Time { return now }
	registrySleep = func(d time.Duration) { now = now.Add(d) }
	defer func() { registryNow, registrySleep = restoreNow, restoreSleep }()

	if _, err := fetchRegistryOLTSWithRetry(srv.URL, ""); err == nil {
		t.Fatal("expected failure once the startup window is exhausted")
	}
}

// REGISTRY_STARTUP_TIMEOUT=0 disables retry: a single attempt, fail-fast.
func TestFetchRegistryOLTSWithRetry_DisabledByZeroTimeout(t *testing.T) {
	var attempts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	t.Setenv("REGISTRY_STARTUP_TIMEOUT", "0")
	restoreSleep := registrySleep
	registrySleep = func(time.Duration) { t.Fatal("must not sleep when retry is disabled") }
	defer func() { registrySleep = restoreSleep }()

	if _, err := fetchRegistryOLTSWithRetry(srv.URL, ""); err == nil {
		t.Fatal("expected fail-fast error with retry disabled")
	}
	if attempts != 1 {
		t.Fatalf("expected exactly 1 attempt with retry disabled, got %d", attempts)
	}
}

func TestRegistryStartupTimeout(t *testing.T) {
	t.Setenv("REGISTRY_STARTUP_TIMEOUT", "")
	if got := registryStartupTimeout(); got != defaultRegistryStartupTimeout {
		t.Fatalf("unset -> default, got %s", got)
	}
	t.Setenv("REGISTRY_STARTUP_TIMEOUT", "90s")
	if got := registryStartupTimeout(); got != 90*time.Second {
		t.Fatalf("90s, got %s", got)
	}
	t.Setenv("REGISTRY_STARTUP_TIMEOUT", "garbage")
	if got := registryStartupTimeout(); got != defaultRegistryStartupTimeout {
		t.Fatalf("unparsable -> default, got %s", got)
	}
}

// FetchWebhookConfig happy path: decodes the device-registry envelope.
func TestFetchWebhookConfig_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/webhook" || r.Header.Get("X-API-Key") != "k" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"data":{"url":"https://hook","type":"telegram","chat_id":"42","enabled":true,"retries":3,"timeout":9}}`))
	}))
	defer srv.Close()

	got, err := FetchWebhookConfig(srv.URL, "k")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if got.URL != "https://hook" || got.Type != "telegram" || got.ChatID != "42" ||
		!got.Enabled || got.Retries != 3 || got.Timeout != 9 {
		t.Fatalf("unexpected webhook config: %+v", got)
	}
}

func TestFetchWebhookConfig_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	if _, err := FetchWebhookConfig(srv.URL, ""); err == nil {
		t.Fatal("expected error on non-200")
	}
}

func TestFetchWebhookConfig_BadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()
	if _, err := FetchWebhookConfig(srv.URL, ""); err == nil {
		t.Fatal("expected decode error")
	}
}

func TestFetchWebhookConfig_BadURL(t *testing.T) {
	// http.NewRequestWithContext fails for a control-char URL — covers the
	// request-build error return.
	if _, err := FetchWebhookConfig("http://\x7f", ""); err == nil {
		t.Fatal("expected request build error")
	}
}

func TestFetchWebhookConfig_Unreachable(t *testing.T) {
	// Do() error path: nothing listening on this port.
	if _, err := FetchWebhookConfig("http://127.0.0.1:1", ""); err == nil {
		t.Fatal("expected transport error")
	}
}
