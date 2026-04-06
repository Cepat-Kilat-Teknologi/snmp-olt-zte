package trap

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/model"
	"github.com/rs/zerolog/log"
)

// WebhookClient sends trap events to external webhook URLs
type WebhookClient struct {
	url        string
	maxRetries int
	timeout    time.Duration
	client     *http.Client
}

// NewWebhookClient creates a new webhook client
func NewWebhookClient(url string, maxRetries int, timeoutSec int) *WebhookClient {
	return &WebhookClient{
		url:        url,
		maxRetries: maxRetries,
		timeout:    time.Duration(timeoutSec) * time.Second,
		client: &http.Client{
			Timeout: time.Duration(timeoutSec) * time.Second,
		},
	}
}

// Send dispatches a trap event to the webhook URL with retry
func (w *WebhookClient) Send(event model.TrapEvent) {
	payload, err := json.Marshal(event)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal trap event for webhook")
		return
	}

	var lastErr error
	for attempt := 0; attempt <= w.maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * time.Second
			log.Warn().
				Int("attempt", attempt).
				Dur("backoff", backoff).
				Msg("Retrying webhook")
			time.Sleep(backoff)
		}

		resp, err := w.client.Post(w.url, "application/json", bytes.NewReader(payload))
		if err != nil {
			lastErr = err
			log.Error().Err(err).
				Int("attempt", attempt).
				Msg("Webhook request failed")
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			log.Info().
				Str("url", w.url).
				Int("status", resp.StatusCode).
				Int("board", event.Board).
				Int("pon", event.PON).
				Int("onu_id", event.OnuID).
				Str("event_type", event.EventType).
				Msg("Webhook sent successfully")
			return
		}

		lastErr = fmt.Errorf("webhook returned status %d", resp.StatusCode)
		log.Error().
			Int("status", resp.StatusCode).
			Int("attempt", attempt).
			Msg("Webhook returned non-2xx status")
	}

	log.Error().Err(lastErr).
		Str("url", w.url).
		Int("retries", w.maxRetries).
		Msg("Webhook failed after all retries")
}
