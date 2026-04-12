package trap

import (
	"context"
	"time"

	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/model"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/pkg/logger"
	"go.uber.org/zap"
)

// offlineEventTypes lists event types that should trigger webhook notifications
var offlineEventTypes = map[string]bool{
	"LOS":        true,
	"DyingGasp":  true,
	"PowerOff":   true,
	"Offline":    true,
	"AuthFailed": true,
	"LOSi":       true,
	"LOFi":       true,
}

// ONUDetailFetcher fetches full ONU detail to enrich trap events
type ONUDetailFetcher interface {
	GetByBoardIDPonIDAndOnuID(ctx context.Context, boardID, ponID, onuID int) (model.ONUCustomerInfo, error)
}

// Handler processes trap events and dispatches notifications
type Handler struct {
	webhook    *WebhookClient
	onuFetcher ONUDetailFetcher
}

// NewHandler creates a new trap event handler
func NewHandler(webhook *WebhookClient, onuFetcher ONUDetailFetcher) *Handler {
	return &Handler{
		webhook:    webhook,
		onuFetcher: onuFetcher,
	}
}

// HandleEvent processes a trap event, enriches with ONU info, and dispatches webhook
func (h *Handler) HandleEvent(event model.TrapEvent) {
	logger.Info("trap_event_received",
		zap.String("source", event.Source),
		zap.Int("board", event.Board),
		zap.Int("pon", event.PON),
		zap.Int("onu_id", event.OnuID),
		zap.String("event_type", event.EventType),
		zap.String("status", event.Status))

	// Only send webhook for offline/alert events
	if !offlineEventTypes[event.EventType] {
		logger.Debug("skipping_non_alert_event", zap.String("event_type", event.EventType))
		return
	}

	// Enrich trap with full ONU detail (name, alamat, serial, rx/tx power, etc.)
	if h.onuFetcher != nil && event.Board > 0 && event.PON > 0 && event.OnuID > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		detail, err := h.onuFetcher.GetByBoardIDPonIDAndOnuID(ctx, event.Board, event.PON, event.OnuID)
		if err == nil && detail.ID > 0 {
			event.Name = detail.Name
			event.Description = detail.Description
			event.OnuType = detail.OnuType
			event.SerialNumber = detail.SerialNumber

			logger.Info("trap_enriched_with_onu_detail",
				zap.String("name", event.Name),
				zap.String("serial", event.SerialNumber),
				zap.String("address", event.Description))
		} else if err != nil {
			logger.Warn("trap_enrich_failed",
				zap.Error(err),
				zap.Int("board", event.Board),
				zap.Int("pon", event.PON),
				zap.Int("onu_id", event.OnuID))
		}
	}

	// Send webhook notification (non-blocking)
	if h.webhook != nil {
		go h.webhook.Send(event)
	}
}
