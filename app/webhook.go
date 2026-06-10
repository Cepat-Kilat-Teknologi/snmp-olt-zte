package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/config"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/model"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/trap"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/pkg/logger"
	"go.uber.org/zap"
)

// whMu guards the two package vars below, which the background webhookRefresher
// goroutine reads while initWebhook / tests write them.
var whMu sync.RWMutex

// Env (TRAP_WEBHOOK_*) fallback, captured at startup. device-registry
// (REGISTRY_URL) is authoritative when set and is read directly from the
// environment on each resolve, so the webhook config + test work even when the
// SNMP trap listener itself is disabled.
var whEnv trap.WebhookSettings

// webhookRefreshInterval is the registry poll cadence for webhookRefresher. It is
// a package var (defaulting to 45s, the production value) so tests can shorten it
// to exercise the hot-swap loop without waiting.
var webhookRefreshInterval = 45 * time.Second

func setWhEnv(s trap.WebhookSettings) { whMu.Lock(); whEnv = s; whMu.Unlock() }
func getWhEnv() trap.WebhookSettings  { whMu.RLock(); defer whMu.RUnlock(); return whEnv }

func setWebhookRefreshInterval(d time.Duration) {
	whMu.Lock()
	webhookRefreshInterval = d
	whMu.Unlock()
}
func getWebhookRefreshInterval() time.Duration {
	whMu.RLock()
	defer whMu.RUnlock()
	return webhookRefreshInterval
}

func regSource() (string, string) {
	return os.Getenv("REGISTRY_URL"), os.Getenv("REGISTRY_API_KEY")
}

func webhookSource() string {
	if u, _ := regSource(); u != "" {
		return "registry"
	}
	return "env"
}

// initWebhook installs the initial webhook client for all trap consumers and,
// when REGISTRY_URL is set, starts a background refresher so edits in the UI
// (device-registry) apply live without a snmp-olt-zte restart.
func initWebhook(env trap.WebhookSettings) {
	setWhEnv(env)

	s := currentWebhookSettings()
	trap.SetActiveWebhook(trap.BuildWebhookClient(s))
	logger.Info("webhook_initialized",
		zap.Bool("enabled", s.Enabled),
		zap.String("source", webhookSource()),
		zap.String("platform", trap.DetectPlatform(s.URL)))

	if u, _ := regSource(); u != "" {
		go webhookRefresher()
	}
}

// currentWebhookSettings returns the registry config when REGISTRY_URL is set
// (falling back to env on error), otherwise the env config.
func currentWebhookSettings() trap.WebhookSettings {
	regURL, regKey := regSource()
	if regURL == "" {
		return getWhEnv()
	}
	r, err := config.FetchWebhookConfig(regURL, regKey)
	if err != nil {
		logger.Warn("webhook_registry_fetch_failed_using_env", zap.Error(err))
		return getWhEnv()
	}
	return trap.WebhookSettings{
		URL: r.URL, Type: r.Type, ChatID: r.ChatID,
		Enabled: r.Enabled, Retries: r.Retries, Timeout: r.Timeout,
	}
}

func settingsKey(s trap.WebhookSettings) string {
	return fmt.Sprintf("%v|%s|%s|%s|%d|%d", s.Enabled, s.URL, s.Type, s.ChatID, s.Retries, s.Timeout)
}

// webhookRefresher polls the registry and hot-swaps the active webhook client
// when the config changes.
func webhookRefresher() {
	t := time.NewTicker(getWebhookRefreshInterval())
	defer t.Stop()
	last := settingsKey(currentWebhookSettings())
	for range t.C {
		s := currentWebhookSettings()
		key := settingsKey(s)
		if key == last {
			continue
		}
		last = key
		trap.SetActiveWebhook(trap.BuildWebhookClient(s))
		logger.Info("webhook_config_reloaded", zap.Bool("enabled", s.Enabled), zap.String("platform", trap.DetectPlatform(s.URL)))
	}
}

// webhookTestHandler sends a test notification using the current config, so the
// UI "Test webhook" button verifies the destination live.
func webhookTestHandler(w http.ResponseWriter, _ *http.Request) {
	s := currentWebhookSettings()
	wc := trap.BuildWebhookClient(s)
	if wc == nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"message": "webhook is disabled or has no URL"})
		return
	}
	wc.Send(model.TrapEvent{
		Timestamp:   time.Now(),
		Source:      "webhook-test",
		EventType:   "test",
		Status:      "test",
		Name:        "Webhook test",
		Description: "Test notification from snmp-olt-zte — your webhook is configured correctly.",
	})
	writeJSONStatus(w, http.StatusOK, map[string]string{"message": "test notification sent"})
}

// webhookNotifyHandler sends a custom notification via the current webhook
// config + platform formatter. Used by the BFF to alert operators on events it
// owns (e.g. a provisioning failure). Best-effort: a disabled webhook is a no-op.
func webhookNotifyHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Status      string `json:"status"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Name == "" {
		body.Name = "Notification"
	}
	if body.Status == "" {
		body.Status = "alert"
	}
	s := currentWebhookSettings()
	wc := trap.BuildWebhookClient(s)
	if wc == nil {
		writeJSONStatus(w, http.StatusOK, map[string]string{"message": "webhook disabled — skipped"})
		return
	}
	wc.Send(model.TrapEvent{
		Timestamp:   time.Now(),
		Source:      "provisioning",
		EventType:   "provisioning",
		Status:      body.Status,
		Name:        body.Name,
		Description: body.Description,
	})
	writeJSONStatus(w, http.StatusOK, map[string]string{"message": "notification sent"})
}
