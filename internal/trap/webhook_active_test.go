package trap

import "testing"

func TestBuildWebhookClient(t *testing.T) {
	if BuildWebhookClient(WebhookSettings{Enabled: false, URL: "https://x"}) != nil {
		t.Error("disabled should yield nil")
	}
	if BuildWebhookClient(WebhookSettings{Enabled: true, URL: ""}) != nil {
		t.Error("empty URL should yield nil")
	}
	c := BuildWebhookClient(WebhookSettings{Enabled: true, URL: "https://discord.com/api/webhooks/abc", Retries: 0, Timeout: 0})
	if c == nil {
		t.Fatal("enabled + URL should build a client")
	}
	if c.maxRetries != 3 || c.timeout.Seconds() != 10 {
		t.Errorf("zero retries/timeout should default to 3/10, got %d/%v", c.maxRetries, c.timeout)
	}
}

func TestActiveWebhookSwap(t *testing.T) {
	// Fully reset global state afterwards so other trap tests still resolve the
	// per-instance client (activeSet must go back to false).
	defer func() {
		activeWebhook.Store(nil)
		activeSet.Store(false)
	}()
	c := BuildWebhookClient(WebhookSettings{Enabled: true, URL: "https://hooks.slack.com/x"})
	SetActiveWebhook(c)
	if ActiveWebhook() != c {
		t.Error("active webhook not installed")
	}
	// resolveWebhook prefers the active client once installed.
	if resolveWebhook(nil) != c {
		t.Error("resolveWebhook should return the active client")
	}
	SetActiveWebhook(nil)
	if ActiveWebhook() != nil || resolveWebhook(nil) != nil {
		t.Error("active webhook not cleared")
	}
}
