package trap

import (
	"context"
	"strings"
	"time"

	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/model"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/pkg/logger"
	"go.uber.org/zap"
)

// alertEventTypes lists event types that should trigger webhook notifications
var alertEventTypes = map[string]bool{
	"LOS":             true,
	"DyingGasp":       true,
	"PowerOff":        true,
	"Offline":         true,
	"AuthFailed":      true,
	"LOSi":            true,
	"LOFi":            true,
	"Logging":         true,
	"Synchronization": true,
}

// ONUDetailFetcher fetches full ONU detail to enrich trap events
type ONUDetailFetcher interface {
	GetByBoardIDPonIDAndOnuID(ctx context.Context, boardID, ponID, onuID int) (model.ONUCustomerInfo, error)
	InvalidateONUCache(ctx context.Context, boardID, ponID, onuID int) error
}

// Handler processes trap events and dispatches notifications
type Handler struct {
	webhook    *WebhookClient
	batcher    *Batcher
	onuFetcher ONUDetailFetcher
}

// NewHandler creates a new trap event handler.
// If batcher is non-nil, events are batched; otherwise sent immediately via webhook.
func NewHandler(webhook *WebhookClient, batcher *Batcher, onuFetcher ONUDetailFetcher) *Handler {
	return &Handler{
		webhook:    webhook,
		batcher:    batcher,
		onuFetcher: onuFetcher,
	}
}

// HandleEvent processes a trap event, verifies ONU status via SNMP,
// and dispatches webhook only for genuinely offline/abnormal ONUs.
func (h *Handler) HandleEvent(event model.TrapEvent) {
	logger.Info("trap_event_received",
		zap.String("source", event.Source),
		zap.Int("board", event.Board),
		zap.Int("pon", event.PON),
		zap.Int("onu_id", event.OnuID),
		zap.String("trap_event_type", event.EventType),
		zap.String("name", event.Name))

	if event.Board == 0 || event.PON == 0 || event.OnuID == 0 {
		logger.Debug("skipping_incomplete_trap_data")
		return
	}

	// Query actual ONU status from SNMP — don't trust trap OID or cache
	if h.onuFetcher != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_ = h.onuFetcher.InvalidateONUCache(ctx, event.Board, event.PON, event.OnuID)

		detail, err := h.onuFetcher.GetByBoardIDPonIDAndOnuID(ctx, event.Board, event.PON, event.OnuID)
		if err != nil {
			logger.Warn("trap_status_verification_failed",
				zap.Error(err),
				zap.Int("board", event.Board),
				zap.Int("pon", event.PON),
				zap.Int("onu_id", event.OnuID))
			return
		}

		if detail.ID == 0 {
			logger.Debug("trap_onu_not_found_in_snmp")
			return
		}

		// Enrich event with SNMP data (prefer SNMP over trap for accuracy)
		if detail.Name != "" {
			event.Name = detail.Name
		}
		if detail.Description != "" {
			event.Description = detail.Description
		}
		event.OnuType = detail.OnuType
		event.SerialNumber = detail.SerialNumber
		event.RXPower = detail.RXPower
		event.LastOffline = detail.LastOffline
		event.LastOnline = detail.LastOnline

		// Determine ACTUAL event type from verified SNMP status
		event.Status = detail.Status
		event.EventType = resolveEventType(detail.Status, detail.LastOfflineReason)

		logger.Info("trap_status_verified",
			zap.String("name", event.Name),
			zap.String("verified_status", detail.Status),
			zap.String("event_type", event.EventType),
			zap.String("offline_reason", detail.LastOfflineReason),
			zap.String("serial", event.SerialNumber))
	}

	// Only alert for confirmed offline/abnormal events
	if !alertEventTypes[event.EventType] {
		if h.batcher != nil {
			h.batcher.Remove(event)
		}
		logger.Debug("skipping_non_alert_event",
			zap.String("event_type", event.EventType),
			zap.String("name", event.Name))
		return
	}

	if h.batcher != nil {
		h.batcher.Add(event)
	} else if wc := resolveWebhook(h.webhook); wc != nil {
		go wc.Send(event)
	}
}

// resolveEventType determines the event type from the actual SNMP-verified
// ONU status and offline reason. Only returns "Online" for confirmed online ONUs.
func resolveEventType(status, offlineReason string) string {
	s := strings.ToLower(status)

	// Explicitly online — skip
	if strings.Contains(s, "online") {
		return "Online"
	}

	// Map known offline/abnormal status strings
	switch {
	case strings.Contains(s, "dying"):
		return "DyingGasp"
	case strings.Contains(s, "los"):
		return "LOS"
	case strings.Contains(s, "power"):
		return "PowerOff"
	case strings.Contains(s, "auth"):
		return "AuthFailed"
	case strings.Contains(s, "logging"):
		return "Logging"
	case strings.Contains(s, "sync"):
		return "Synchronization"
	case strings.Contains(s, "offline"):
		if offlineReason != "" {
			return normalizeOfflineReason(offlineReason)
		}
		return "Offline"
	default:
		return "Online"
	}
}

// normalizeOfflineReason maps known offline reason strings to event types.
func normalizeOfflineReason(reason string) string {
	r := strings.ToLower(reason)
	switch {
	case strings.Contains(r, "losi"):
		return "LOSi"
	case strings.Contains(r, "lofi"):
		return "LOFi"
	case strings.Contains(r, "los"):
		return "LOS"
	case strings.Contains(r, "dyinggasp"), strings.Contains(r, "dying"):
		return "DyingGasp"
	case strings.Contains(r, "poweroff"), strings.Contains(r, "power"):
		return "PowerOff"
	case strings.Contains(r, "authfail"), strings.Contains(r, "auth"):
		return "AuthFailed"
	default:
		return "Offline"
	}
}
