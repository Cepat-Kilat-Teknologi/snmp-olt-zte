package trap

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/model"
	"github.com/rs/zerolog/log"
)

// ONUListFetcher fetches the cached ONU list for a board/PON
type ONUListFetcher interface {
	GetByBoardIDAndPonID(ctx context.Context, boardID, ponID int) ([]model.ONUInfoPerBoard, error)
}

// PowerMonitorConfig holds configuration for the power monitor
type PowerMonitorConfig struct {
	Interval      time.Duration
	HighThreshold float64 // dBm, above this = overload alert
	LowThreshold  float64 // dBm, below this = weak signal alert
	Source        string  // OLT IP for event source field
}

// PowerMonitor periodically checks ONU RX power levels and sends alerts
type PowerMonitor struct {
	config     PowerMonitorConfig
	fetcher    ONUListFetcher
	webhook    *WebhookClient
	stopCh     chan struct{}
	alerted    map[string]time.Time // track alerted ONUs to avoid spam
}

// NewPowerMonitor creates a new power monitor
func NewPowerMonitor(cfg PowerMonitorConfig, fetcher ONUListFetcher, webhook *WebhookClient) *PowerMonitor {
	return &PowerMonitor{
		config:  cfg,
		fetcher: fetcher,
		webhook: webhook,
		stopCh:  make(chan struct{}),
		alerted: make(map[string]time.Time),
	}
}

// Start begins the periodic power monitoring loop
func (pm *PowerMonitor) Start() {
	log.Info().
		Dur("interval", pm.config.Interval).
		Float64("high_threshold", pm.config.HighThreshold).
		Float64("low_threshold", pm.config.LowThreshold).
		Msg("Starting RX Power monitor")

	ticker := time.NewTicker(pm.config.Interval)
	defer ticker.Stop()

	// Run immediately on start, then on interval
	pm.scan()

	for {
		select {
		case <-ticker.C:
			pm.scan()
		case <-pm.stopCh:
			log.Info().Msg("Power monitor stopped")
			return
		}
	}
}

// Close stops the power monitor
func (pm *PowerMonitor) Close() error {
	close(pm.stopCh)
	return nil
}

// scan checks all board/PON combinations for abnormal RX power
func (pm *PowerMonitor) scan() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	log.Debug().Msg("Power monitor: scanning all PONs")

	alertCount := 0
	for boardID := 1; boardID <= 2; boardID++ {
		for ponID := 1; ponID <= 16; ponID++ {
			onus, err := pm.fetcher.GetByBoardIDAndPonID(ctx, boardID, ponID)
			if err != nil {
				continue // skip PONs that fail (may not have ONUs)
			}

			for _, onu := range onus {
				if onu.RXPower == "" {
					continue
				}

				power, err := strconv.ParseFloat(onu.RXPower, 64)
				if err != nil {
					continue
				}

				alertKey := fmt.Sprintf("%d-%d-%d", boardID, ponID, onu.ID)

				// Check high power (overload)
				if power > pm.config.HighThreshold {
					if pm.shouldAlert(alertKey) {
						pm.sendAlert(onu, "HighRxPower", fmt.Sprintf(
							"ONU %d/%d/%d RX Power %.2f dBm exceeds high threshold %.1f dBm (overload risk)",
							boardID, ponID, onu.ID, power, pm.config.HighThreshold,
						))
						alertCount++
					}
					continue
				}

				// Check low power (weak signal, approaching LOS)
				if power < pm.config.LowThreshold {
					if pm.shouldAlert(alertKey) {
						pm.sendAlert(onu, "LowRxPower", fmt.Sprintf(
							"ONU %d/%d/%d RX Power %.2f dBm below low threshold %.1f dBm (weak signal)",
							boardID, ponID, onu.ID, power, pm.config.LowThreshold,
						))
						alertCount++
					}
					continue
				}

				// Power is normal — clear alert state
				delete(pm.alerted, alertKey)
			}
		}
	}

	if alertCount > 0 {
		log.Warn().Int("alerts", alertCount).Msg("Power monitor: abnormal RX power detected")
	} else {
		log.Debug().Msg("Power monitor: scan complete, all power levels normal")
	}
}

// shouldAlert checks if we should send an alert (avoid spamming same ONU)
// Only alert once per 30 minutes per ONU
func (pm *PowerMonitor) shouldAlert(key string) bool {
	if lastAlert, exists := pm.alerted[key]; exists {
		if time.Since(lastAlert) < 30*time.Minute {
			return false
		}
	}
	pm.alerted[key] = time.Now()
	return true
}

// sendAlert creates a TrapEvent and sends via webhook
func (pm *PowerMonitor) sendAlert(onu model.ONUInfoPerBoard, eventType, description string) {
	event := model.TrapEvent{
		Timestamp:    time.Now(),
		Source:       pm.config.Source,
		Board:        onu.Board,
		PON:          onu.PON,
		OnuID:        onu.ID,
		EventType:    eventType,
		Status:       onu.Status,
		Name:         onu.Name,
		OnuType:      onu.OnuType,
		SerialNumber: onu.SerialNumber,
		RXPower:      onu.RXPower,
		Description:  description,
	}

	log.Warn().
		Int("board", onu.Board).
		Int("pon", onu.PON).
		Int("onu_id", onu.ID).
		Str("rx_power", onu.RXPower).
		Str("event_type", eventType).
		Str("name", onu.Name).
		Msg("RX Power alert")

	if pm.webhook != nil {
		go pm.webhook.Send(event)
	}
}
