package trap

import "sync/atomic"

// The "active" webhook client is a single, atomically-swappable pointer that all
// trap consumers (batcher, handler, power monitor) read at send time. A
// background refresher swaps it when the device-registry webhook config changes,
// so notification settings apply live without a restart. nil = no/disabled
// webhook (consumers skip sending).
var (
	activeWebhook atomic.Pointer[WebhookClient]
	activeSet     atomic.Bool // true once the app installs a live config source
)

// SetActiveWebhook installs the webhook client used by all trap consumers. Once
// called (even with nil, e.g. webhook disabled), the active client is
// authoritative over each consumer's construction-time client.
func SetActiveWebhook(c *WebhookClient) {
	activeWebhook.Store(c)
	activeSet.Store(true)
}

// ActiveWebhook returns the currently-active webhook client (or nil).
func ActiveWebhook() *WebhookClient { return activeWebhook.Load() }

// resolveWebhook returns the live active client when the app has installed one,
// otherwise the consumer's construction-time client (used by unit tests that
// wire a client directly without a registry/env source).
func resolveWebhook(instance *WebhookClient) *WebhookClient {
	if activeSet.Load() {
		return activeWebhook.Load()
	}
	return instance
}

// WebhookSettings is the resolved webhook configuration (from device-registry or
// env) used to (re)build a client.
type WebhookSettings struct {
	URL     string
	Type    string
	ChatID  string
	Enabled bool
	Retries int
	Timeout int
}

// BuildWebhookClient builds a client for s, or nil when disabled / no URL.
func BuildWebhookClient(s WebhookSettings) *WebhookClient {
	if !s.Enabled || s.URL == "" {
		return nil
	}
	retries, timeout := s.Retries, s.Timeout
	if retries <= 0 {
		retries = 3
	}
	if timeout <= 0 {
		timeout = 10
	}
	formatter, finalURL := NewFormatter(s.URL, s.Type, s.ChatID)
	return NewWebhookClient(finalURL, retries, timeout, formatter)
}
