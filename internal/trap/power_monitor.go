package trap

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/model"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

// ONUListFetcher fetches the cached ONU list for a board/PON
type ONUListFetcher interface {
	GetByBoardIDAndPonID(ctx context.Context, boardID, ponID int) ([]model.ONUInfoPerBoard, error)
}

// PowerMonitorConfig holds configuration for the power monitor
type PowerMonitorConfig struct {
	Interval      time.Duration
	Cron          string // cron expression (5-field), e.g. "0 8,12,15,17,0 * * *"
	Timezone      string // IANA timezone, e.g. "Asia/Jakarta" (empty = local)
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
	mu         sync.Mutex
	alerted    map[string]time.Time // track alerted ONUs to avoid spam
	cronRunner *cron.Cron           // nil if cron not configured
	closeOnce  sync.Once
}

// NewPowerMonitor creates a new power monitor
func NewPowerMonitor(cfg PowerMonitorConfig, fetcher ONUListFetcher, webhook *WebhookClient) *PowerMonitor {
	pm := &PowerMonitor{
		config:  cfg,
		fetcher: fetcher,
		webhook: webhook,
		stopCh:  make(chan struct{}),
		alerted: make(map[string]time.Time),
	}

	// Setup cron scheduler if cron expression is provided
	if cfg.Cron != "" {
		var opts []cron.Option

		// Parse timezone
		if cfg.Timezone != "" {
			loc, err := time.LoadLocation(cfg.Timezone)
			if err != nil {
				log.Warn().Str("timezone", cfg.Timezone).Err(err).
					Msg("Invalid timezone, falling back to local")
			} else {
				opts = append(opts, cron.WithLocation(loc))
			}
		}

		cronRunner := cron.New(opts...)
		_, err := cronRunner.AddFunc(cfg.Cron, pm.safeScan)
		if err != nil {
			log.Error().Str("cron", cfg.Cron).Err(err).
				Msg("Invalid cron expression, cron scheduling disabled")
		} else {
			pm.cronRunner = cronRunner
		}
	}

	return pm
}

// Start begins the periodic power monitoring loop
func (pm *PowerMonitor) Start() {
	hasInterval := pm.config.Interval > 0
	hasCron := pm.cronRunner != nil

	if !hasInterval && !hasCron {
		log.Warn().Msg("Power monitor: both interval and cron are disabled, nothing to do")
		return
	}

	log.Info().
		Dur("interval", pm.config.Interval).
		Str("cron", pm.config.Cron).
		Str("timezone", pm.config.Timezone).
		Float64("high_threshold", pm.config.HighThreshold).
		Float64("low_threshold", pm.config.LowThreshold).
		Msg("Starting RX Power monitor")

	// Start cron scheduler if configured
	if hasCron {
		pm.cronRunner.Start()
		log.Info().Str("cron", pm.config.Cron).Msg("Power monitor: cron scheduler started")
	}

	// Run interval ticker if configured
	if hasInterval {
		ticker := time.NewTicker(pm.config.Interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				pm.safeScan()
			case <-pm.stopCh:
				log.Info().Msg("Power monitor stopped")
				return
			}
		}
	} else {
		// Cron-only mode: block until stop signal
		<-pm.stopCh
		log.Info().Msg("Power monitor stopped")
	}
}

// Close stops the power monitor
func (pm *PowerMonitor) Close() error {
	pm.closeOnce.Do(func() {
		if pm.cronRunner != nil {
			pm.cronRunner.Stop()
		}
		close(pm.stopCh)
	})
	return nil
}

// safeScan wraps scan with panic recovery to prevent goroutine crashes
func (pm *PowerMonitor) safeScan() {
	defer func() {
		if r := recover(); r != nil {
			log.Error().Interface("panic", r).Msg("Power monitor: scan panicked, recovered")
		}
	}()
	pm.scan()
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
				pm.mu.Lock()
				delete(pm.alerted, alertKey)
				pm.mu.Unlock()
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
	pm.mu.Lock()
	defer pm.mu.Unlock()
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
