package trap

import (
	"context"
	"time"

	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/model"
	"github.com/rs/zerolog/log"
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
	log.Info().
		Str("source", event.Source).
		Int("board", event.Board).
		Int("pon", event.PON).
		Int("onu_id", event.OnuID).
		Str("event_type", event.EventType).
		Str("status", event.Status).
		Msg("Trap event received")

	// Only send webhook for offline/alert events
	if !offlineEventTypes[event.EventType] {
		log.Debug().Str("event_type", event.EventType).Msg("Skipping non-alert event")
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

			log.Info().
				Str("name", event.Name).
				Str("serial", event.SerialNumber).
				Str("address", event.Description).
				Msg("Trap enriched with ONU detail")
		} else if err != nil {
			log.Warn().Err(err).
				Int("board", event.Board).Int("pon", event.PON).Int("onu_id", event.OnuID).
				Msg("Failed to enrich trap with ONU detail")
		}
	}

	// Send webhook notification (non-blocking)
	if h.webhook != nil {
		go h.webhook.Send(event)
	}
}
