package app

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/trap"
)

func TestWebhookSource(t *testing.T) {
	t.Setenv("REGISTRY_URL", "")
	if got := webhookSource(); got != "env" {
		t.Errorf("webhookSource()=%q want env", got)
	}
	t.Setenv("REGISTRY_URL", "http://reg")
	if got := webhookSource(); got != "registry" {
		t.Errorf("webhookSource()=%q want registry", got)
	}
}

func TestSettingsKey(t *testing.T) {
	a := trap.WebhookSettings{Enabled: true, URL: "u", Type: "telegram", ChatID: "1", Retries: 3, Timeout: 9}
	b := a
	if settingsKey(a) != settingsKey(b) {
		t.Fatal("identical settings should produce identical keys")
	}
	b.URL = "v"
	if settingsKey(a) == settingsKey(b) {
		t.Fatal("different URL should change the key")
	}
}

func TestCurrentWebhookSettings_Env(t *testing.T) {
	t.Setenv("REGISTRY_URL", "")
	setWhEnv(trap.WebhookSettings{URL: "http://env", Enabled: true})
	got := currentWebhookSettings()
	if got.URL != "http://env" || !got.Enabled {
		t.Fatalf("env path: got %+v", got)
	}
}

func TestCurrentWebhookSettings_Registry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"url":"http://reg","type":"discord","chat_id":"","enabled":true,"retries":2,"timeout":7}}`))
	}))
	defer srv.Close()
	t.Setenv("REGISTRY_URL", srv.URL)
	t.Setenv("REGISTRY_API_KEY", "")

	got := currentWebhookSettings()
	if got.URL != "http://reg" || got.Type != "discord" || !got.Enabled || got.Retries != 2 || got.Timeout != 7 {
		t.Fatalf("registry path: got %+v", got)
	}
}

func TestCurrentWebhookSettings_RegistryErrorFallsBackToEnv(t *testing.T) {
	t.Setenv("REGISTRY_URL", "http://127.0.0.1:1") // unreachable -> fetch error
	t.Setenv("REGISTRY_API_KEY", "")
	setWhEnv(trap.WebhookSettings{URL: "http://fallback", Enabled: false})

	got := currentWebhookSettings()
	if got.URL != "http://fallback" {
		t.Fatalf("expected env fallback on registry error, got %+v", got)
	}
}

func TestInitWebhook_EnvOnly(t *testing.T) {
	t.Setenv("REGISTRY_URL", "") // no refresher goroutine spawned
	initWebhook(trap.WebhookSettings{URL: "http://env", Enabled: true})
	if getWhEnv().URL != "http://env" {
		t.Fatalf("initWebhook did not capture env settings: %+v", getWhEnv())
	}
}

func TestInitWebhook_RegistrySpawnsRefresher(t *testing.T) {
	// Registry returns a config that changes after the first poll so the refresher
	// loop hits its hot-swap branch.
	var n int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n++
		url := "http://reg-a"
		if n > 1 {
			url = "http://reg-b" // change -> triggers reload branch
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"url": url, "enabled": true, "retries": 3, "timeout": 5},
		})
	}))
	defer srv.Close()

	t.Setenv("REGISTRY_URL", srv.URL)
	t.Setenv("REGISTRY_API_KEY", "")

	orig := getWebhookRefreshInterval()
	setWebhookRefreshInterval(10 * time.Millisecond)
	defer setWebhookRefreshInterval(orig)

	initWebhook(trap.WebhookSettings{URL: "http://env", Enabled: true})

	// Give the refresher a few ticks to poll + hot-swap.
	deadline := time.After(2 * time.Second)
	for {
		if c := trap.ActiveWebhook(); c != nil {
			break
		}
		select {
		case <-deadline:
			t.Fatal("active webhook never installed by refresher path")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestWebhookTestHandler_Disabled(t *testing.T) {
	t.Setenv("REGISTRY_URL", "")
	setWhEnv(trap.WebhookSettings{URL: "", Enabled: false}) // BuildWebhookClient -> nil

	req := httptest.NewRequest("POST", "/api/v1/webhook/test", nil)
	rr := httptest.NewRecorder()
	webhookTestHandler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("disabled webhook: status=%d want 400", rr.Code)
	}
}

func TestWebhookTestHandler_Sends(t *testing.T) {
	// Webhook target captures the test notification so the handler reaches its 200.
	hit := make(chan struct{}, 1)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		select {
		case hit <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	t.Setenv("REGISTRY_URL", "")
	setWhEnv(trap.WebhookSettings{URL: target.URL, Type: "generic", Enabled: true, Retries: 1, Timeout: 2})

	req := httptest.NewRequest("POST", "/api/v1/webhook/test", nil)
	rr := httptest.NewRecorder()
	webhookTestHandler(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("enabled webhook: status=%d want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "test notification sent") {
		t.Fatalf("unexpected body: %s", rr.Body.String())
	}
}

func TestWebhookNotifyHandler_Disabled(t *testing.T) {
	t.Setenv("REGISTRY_URL", "")
	setWhEnv(trap.WebhookSettings{URL: "", Enabled: false}) // BuildWebhookClient -> nil
	req := httptest.NewRequest("POST", "/api/v1/webhook/notify", strings.NewReader(`{"name":"X","description":"y","status":"error"}`))
	rr := httptest.NewRecorder()
	webhookNotifyHandler(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "skipped") {
		t.Fatalf("disabled notify: status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestWebhookNotifyHandler_Sends(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	t.Setenv("REGISTRY_URL", "")
	setWhEnv(trap.WebhookSettings{URL: target.URL, Type: "generic", Enabled: true, Retries: 1, Timeout: 2})

	// Empty body exercises the name/status default branches.
	req := httptest.NewRequest("POST", "/api/v1/webhook/notify", strings.NewReader(``))
	rr := httptest.NewRecorder()
	webhookNotifyHandler(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "notification sent") {
		t.Fatalf("enabled notify: status=%d body=%s", rr.Code, rr.Body.String())
	}
}

// --- snmpTestHandler ---

func TestSnmpTestHandler_BadJSON(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/v1/test", strings.NewReader("not json"))
	rr := httptest.NewRecorder()
	snmpTestHandler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("bad json: status=%d want 400", rr.Code)
	}
}

func TestSnmpTestHandler_MissingFields(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/v1/test", strings.NewReader(`{"host":"","community":""}`))
	rr := httptest.NewRecorder()
	snmpTestHandler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("missing fields: status=%d want 400", rr.Code)
	}
}

func TestSnmpTestHandler_SetupFailure(t *testing.T) {
	// Invalid host makes SetupSnmpConnectionWith fail (Connect error) -> 502.
	req := httptest.NewRequest("POST", "/api/v1/test",
		strings.NewReader(`{"host":"invalid-host-!@#","community":"public"}`))
	rr := httptest.NewRecorder()
	snmpTestHandler(rr, req)
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("setup failure: status=%d want 502", rr.Code)
	}
}

func TestSnmpTestHandler_GetFailure(t *testing.T) {
	// A bound but silent UDP socket: Connect succeeds (default port branch via
	// port omitted -> 161 would fail; supply the real port), but Get times out ->
	// 502. Use short retries via env to keep the test fast.
	t.Setenv("SNMP_TIMEOUT_SECONDS", "1")
	t.Setenv("SNMP_RETRIES", "0")

	udp, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("udp: %v", err)
	}
	defer func() { _ = udp.Close() }()
	addr := udp.LocalAddr().(*net.UDPAddr)

	body := `{"host":"127.0.0.1","port":` + itoa(addr.Port) + `,"community":"public"}`
	req := httptest.NewRequest("POST", "/api/v1/test", strings.NewReader(body))
	rr := httptest.NewRecorder()
	snmpTestHandler(rr, req)
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("get failure: status=%d want 502", rr.Code)
	}
}

func itoa(i int) string {
	return strings.TrimSpace(jsonInt(i))
}

func jsonInt(i int) string {
	b, _ := json.Marshal(i)
	return string(b)
}

func TestWebhookRefresher_HotReloadAndSteadyState(t *testing.T) {
	// No REGISTRY_URL -> currentWebhookSettings serves the env settings, which
	// this test flips to drive both loop branches: the steady-state `continue`
	// (key unchanged) and the hot-swap reload (key changed).
	t.Setenv("REGISTRY_URL", "")

	orig := getWebhookRefreshInterval()
	setWebhookRefreshInterval(5 * time.Millisecond)
	defer setWebhookRefreshInterval(orig)

	origEnv := getWhEnv()
	defer setWhEnv(origEnv)

	setWhEnv(trap.WebhookSettings{URL: "http://hook-a", Enabled: true, Retries: 1, Timeout: 1})
	before := trap.ActiveWebhook()

	go webhookRefresher()

	// A few unchanged ticks first (steady-state branch)...
	time.Sleep(25 * time.Millisecond)
	// ...then flip the settings so the reload branch installs a new client.
	setWhEnv(trap.WebhookSettings{URL: "http://hook-b", Enabled: true, Retries: 2, Timeout: 2})

	deadline := time.After(2 * time.Second)
	for {
		if c := trap.ActiveWebhook(); c != nil && c != before {
			return // hot-swap observed
		}
		select {
		case <-deadline:
			t.Fatal("refresher never hot-swapped the webhook client")
		case <-time.After(5 * time.Millisecond):
		}
	}
}
