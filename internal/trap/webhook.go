package trap

import (
	"bytes"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/model"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/pkg/logger"
	"go.uber.org/zap"
)

// WebhookClient sends trap events to external webhook URLs
type WebhookClient struct {
	url        string
	maxRetries int
	timeout    time.Duration
	client     *http.Client
	formatter  WebhookFormatter
}

// NewWebhookClient creates a new webhook client with a platform-specific formatter.
func NewWebhookClient(url string, maxRetries int, timeoutSec int, formatter WebhookFormatter) *WebhookClient {
	if formatter == nil {
		formatter = &GenericFormatter{}
	}
	return &WebhookClient{
		url:        url,
		maxRetries: maxRetries,
		timeout:    time.Duration(timeoutSec) * time.Second,
		client: &http.Client{
			Timeout: time.Duration(timeoutSec) * time.Second,
		},
		formatter: formatter,
	}
}

// Send dispatches a trap event to the webhook URL with retry
func (w *WebhookClient) Send(event model.TrapEvent) {
	payload, err := w.formatter.Format(event)
	if err != nil {
		logger.Error("webhook_format_failed", zap.Error(err))
		return
	}

	w.sendPayload(payload)
}

// sendPayload sends raw payload bytes to the webhook URL with retry
func (w *WebhookClient) sendPayload(payload []byte) {
	contentType := w.formatter.ContentType()

	var lastErr error
	for attempt := 0; attempt <= w.maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * time.Second
			logger.Warn("webhook_retry",
				zap.Int("attempt", attempt),
				zap.Duration("backoff", backoff))
			time.Sleep(backoff)
		}

		resp, err := w.client.Post(w.url, contentType, bytes.NewReader(payload))
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
				zap.Int("payload_bytes", len(payload)))
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
