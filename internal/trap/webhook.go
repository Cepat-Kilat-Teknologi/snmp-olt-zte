package trap

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/model"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/pkg/logger"
	"go.uber.org/zap"
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
		logger.Error("webhook_marshal_failed", zap.Error(err))
		return
	}

	var lastErr error
	for attempt := 0; attempt <= w.maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * time.Second
			logger.Warn("webhook_retry",
				zap.Int("attempt", attempt),
				zap.Duration("backoff", backoff))
			time.Sleep(backoff)
		}

		resp, err := w.client.Post(w.url, "application/json", bytes.NewReader(payload))
		if err != nil {
			lastErr = err
			logger.Error("webhook_request_failed",
				zap.Error(err),
				zap.Int("attempt", attempt))
			continue
		}
		_ = resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			logger.Info("webhook_sent_successfully",
				zap.String("url", w.url),
				zap.Int("status", resp.StatusCode),
				zap.Int("board", event.Board),
				zap.Int("pon", event.PON),
				zap.Int("onu_id", event.OnuID),
				zap.String("event_type", event.EventType))
			return
		}

		lastErr = fmt.Errorf("webhook returned status %d", resp.StatusCode)
		logger.Error("webhook_non_2xx_status",
			zap.Int("status", resp.StatusCode),
			zap.Int("attempt", attempt))
	}

	logger.Error("webhook_failed_after_all_retries",
		zap.Error(lastErr),
		zap.String("url", w.url),
		zap.Int("retries", w.maxRetries))
}
