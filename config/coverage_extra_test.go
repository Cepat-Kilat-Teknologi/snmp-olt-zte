package config

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// parseBoardSpecs clamps an out-of-range defaultPons back to MaxPonID. Driving a
// defaultPons of 0 (and 99) covers config.go:180 (the clamp branch).
func TestParseBoardSpecs_DefaultPonsOutOfRange(t *testing.T) {
	for _, dp := range []int{0, 99} {
		_, specs := parseBoardSpecs("3", dp)
		if specs[3] != MaxPonID {
			t.Fatalf("defaultPons=%d: expected slot 3 PON count clamped to %d, got %d", dp, MaxPonID, specs[3])
		}
	}
}

// InitializeBoardPonMapFromSpecs clamps a per-slot PON count that is out of range
// to MaxPonID (oid_generator.go:132).
func TestInitializeBoardPonMapFromSpecs_PonCountClamped(t *testing.T) {
	m, err := InitializeBoardPonMapFromSpecs(map[int]int{2: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 0 clamps to MaxPonID, so every PON 1..MaxPonID must be present for slot 2.
	for pon := 1; pon <= MaxPonID; pon++ {
		if _, ok := m[BoardPonKey{BoardID: 2, PonID: pon}]; !ok {
			t.Fatalf("expected slot 2 pon %d in map after clamp", pon)
		}
	}
}

// InitializeBoardPonMapFromSpecs surfaces the GenerateBoardPonOID error for an
// out-of-range slot (oid_generator.go:138 error wrap).
func TestInitializeBoardPonMapFromSpecs_InvalidSlot(t *testing.T) {
	_, err := InitializeBoardPonMapFromSpecs(map[int]int{MaxBoardID + 1: 4})
	if err == nil {
		t.Fatal("expected error for out-of-range slot")
	}
	if !strings.Contains(err.Error(), "board") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ValidateConfig clamps an out-of-range per-slot PON count to MaxPonID
// (config.go:422) before checking the map.
func TestValidateConfig_PonCountClamped(t *testing.T) {
	m, err := InitializeBoardPonMapFromSpecs(map[int]int{1: MaxPonID})
	if err != nil {
		t.Fatalf("init map: %v", err)
	}
	// BoardPons declares an out-of-range count (99) which ValidateConfig clamps to
	// MaxPonID; the map above has all MaxPonID PONs so validation must pass.
	c := &Config{BoardPons: map[int]int{1: 99}, BoardPonMap: m}
	if err := c.ValidateConfig(); err != nil {
		t.Fatalf("expected validation to pass after clamp, got: %v", err)
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

// LoadConfig falls through to REGISTRY_URL when OLTS/OLTS_FILE are empty, and the
// registry view becomes the OLT registry (config.go:346-351).
func TestLoadConfig_FromRegistryURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"reg-1","host":"10.9.9.9","port":161,"community":"pub","boards":"1,2"}]}`))
	}))
	defer srv.Close()

	t.Setenv("OLTS", "")
	t.Setenv("OLTS_FILE", "")
	t.Setenv("SNMP_HOST", "")
	t.Setenv("SNMP_COMMUNITY", "")
	t.Setenv("REGISTRY_URL", srv.URL)
	t.Setenv("REGISTRY_API_KEY", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if len(cfg.OLTs) != 1 || cfg.OLTs[0].ID != "reg-1" || cfg.DefaultOLT != "reg-1" {
		t.Fatalf("expected registry-derived OLT, got %+v default=%s", cfg.OLTs, cfg.DefaultOLT)
	}
}

// LoadConfig is fail-fast when REGISTRY_URL is set but unreachable (config.go:348).
func TestLoadConfig_RegistryURLError(t *testing.T) {
	t.Setenv("OLTS", "")
	t.Setenv("OLTS_FILE", "")
	t.Setenv("SNMP_HOST", "")
	t.Setenv("SNMP_COMMUNITY", "")
	t.Setenv("REGISTRY_URL", "http://127.0.0.1:1")

	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected fail-fast error for unreachable REGISTRY_URL")
	}
}
