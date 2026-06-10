package config

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// defaultRegistryStartupTimeout bounds how long fetchRegistryOLTSWithRetry waits
// for device-registry to become reachable at startup. During a cold cluster start
// the snmp service frequently boots before device-registry is Ready; without a
// retry it would crash (CrashLoopBackOff inflates restart counts and looks like an
// outage). We do NOT fall back to static config on failure — snmp-olt-zte builds
// per-OLT SNMP pools once at startup with no runtime registry refresher, so a
// silent fallback would pin the process to wrong config. Instead we retry within
// this window and, only if it is exhausted, fail loudly. Override with
// REGISTRY_STARTUP_TIMEOUT (Go duration, e.g. "90s"); set "0" to disable retry.
const defaultRegistryStartupTimeout = 60 * time.Second

// registrySleep is indirected so tests can avoid real backoff sleeps.
var registrySleep = time.Sleep

// registryNow is indirected so tests can drive the deadline deterministically.
var registryNow = time.Now

// registryStartupTimeout reads REGISTRY_STARTUP_TIMEOUT (Go duration) or returns
// the default. A non-positive / unparsable value falls back to the default;
// an explicit "0" disables retry (single attempt, fail-fast).
func registryStartupTimeout() time.Duration {
	v := getEnv("REGISTRY_STARTUP_TIMEOUT", "")
	if v == "" {
		return defaultRegistryStartupTimeout
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return defaultRegistryStartupTimeout
	}
	return d // may be 0 (disable retry) — honored by the loop below
}

// fetchRegistryOLTSWithRetry calls fetchRegistryOLTS, retrying transient failures
// (registry not yet reachable / 5xx) with exponential backoff until the startup
// window elapses. It never falls back to static config; if the window is
// exhausted it returns the last error so a genuinely-misconfigured registry still
// fails fast. The happy path (registry already up) returns on the first attempt.
func fetchRegistryOLTSWithRetry(baseURL, apiKey string) (string, error) {
	const (
		initialBackoff = 1 * time.Second
		maxBackoff     = 8 * time.Second
	)
	deadline := registryNow().Add(registryStartupTimeout())
	backoff := initialBackoff
	var lastErr error
	for {
		olts, err := fetchRegistryOLTS(baseURL, apiKey)
		if err == nil {
			return olts, nil
		}
		lastErr = err
		// Stop if the next sleep would overrun the window (also covers the
		// disable-retry case where the window is already in the past).
		if !registryNow().Add(backoff).Before(deadline) {
			return "", fmt.Errorf("device-registry unreachable, gave up after startup window: %w", lastErr)
		}
		registrySleep(backoff)
		if backoff < maxBackoff {
			if backoff *= 2; backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

// fetchRegistryOLTS GETs the SNMP-scoped OLT view from device-registry
// (/v1/registry/snmp) and returns it as an OLTS JSON array string, ready for
// buildOLTRegistry. The view shape matches oltJSON exactly. An empty inventory
// returns "" so the caller falls back to legacy mode. This makes device-registry
// the single source of truth — no OLTS_FILE to edit when adding an OLT (a
// snmp-olt-zte restart picks up the new device).
func fetchRegistryOLTS(baseURL, apiKey string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1/registry/snmp", http.NoBody)
	if err != nil {
		return "", err
	}
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("device-registry returned %d", resp.StatusCode)
	}

	var env struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return "", fmt.Errorf("decode snmp view: %w", err)
	}
	if len(env.Data) == 0 {
		return "", nil
	}
	b, err := json.Marshal(env.Data)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// WebhookRemote is the trap-notification webhook config stored in device-registry
// (GET /v1/webhook). snmp-olt-zte reads it live so notification settings can be edited
// in the UI without a restart.
type WebhookRemote struct {
	URL     string `json:"url"`
	Type    string `json:"type"`
	ChatID  string `json:"chat_id"`
	Enabled bool   `json:"enabled"`
	Retries int    `json:"retries"`
	Timeout int    `json:"timeout"`
}

// FetchWebhookConfig GETs the global webhook config from device-registry.
func FetchWebhookConfig(baseURL, apiKey string) (WebhookRemote, error) {
	var out WebhookRemote
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1/webhook", http.NoBody)
	if err != nil {
		return out, err
	}
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return out, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return out, fmt.Errorf("device-registry returned %d", resp.StatusCode)
	}
	var env struct {
		Data WebhookRemote `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return out, fmt.Errorf("decode webhook config: %w", err)
	}
	return env.Data, nil
}
