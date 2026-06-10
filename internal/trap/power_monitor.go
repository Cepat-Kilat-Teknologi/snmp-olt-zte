package trap

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/model"
	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/pkg/logger"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// ONUListFetcher fetches the cached ONU list for a board/PON
type ONUListFetcher interface {
	GetByBoardIDAndPonID(ctx context.Context, boardID, ponID int) ([]model.ONUInfoPerBoard, error)
}

// PowerMonitorConfig holds configuration for the power monitor
type PowerMonitorConfig struct {
	Interval      time.Duration
	Cron          string  // cron expression (5-field), e.g. "0 8,12,15,17,0 * * *"
	Timezone      string  // IANA timezone, e.g. "Asia/Jakarta" (empty = local)
	HighThreshold float64 // dBm, above this = overload alert
	LowThreshold  float64 // dBm, below this = weak signal alert
	Source        string  // OLT IP for event source field
	Boards        []int   // physical GPON slots to scan (C320 -> {1,2}, C300 -> e.g. {3,5}); empty = {1,2}
	PonsPerBoard  int     // PON ports per card to scan; <1 = 16
}

// PowerMonitor periodically checks ONU RX power levels and sends alerts
type PowerMonitor struct {
	config     PowerMonitorConfig
	fetcher    ONUListFetcher
	webhook    *WebhookClient
	batcher    *Batcher
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
				logger.Warn("invalid_timezone_fallback_local",
					zap.String("timezone", cfg.Timezone),
					zap.Error(err))
			} else {
				opts = append(opts, cron.WithLocation(loc))
			}
		}

		cronRunner := cron.New(opts...)
		_, err := cronRunner.AddFunc(cfg.Cron, pm.safeScan)
		if err != nil {
			logger.Error("invalid_cron_expression_scheduling_disabled",
				zap.String("cron", cfg.Cron),
				zap.Error(err))
		} else {
			pm.cronRunner = cronRunner
		}
	}

	return pm
}

// SetBatcher routes power alerts through the batcher instead of direct webhook.
func (pm *PowerMonitor) SetBatcher(b *Batcher) {
	pm.batcher = b
}

// Start begins the periodic power monitoring loop
func (pm *PowerMonitor) Start() {
	hasInterval := pm.config.Interval > 0
	hasCron := pm.cronRunner != nil

	if !hasInterval && !hasCron {
		logger.Warn("power_monitor_disabled_nothing_to_do")
		return
	}

	logger.Info("starting_rx_power_monitor",
		zap.Duration("interval", pm.config.Interval),
		zap.String("cron", pm.config.Cron),
		zap.String("timezone", pm.config.Timezone),
		zap.Float64("high_threshold", pm.config.HighThreshold),
		zap.Float64("low_threshold", pm.config.LowThreshold))

	// Start cron scheduler if configured
	if hasCron {
		pm.cronRunner.Start()
		logger.Info("power_monitor_cron_scheduler_started", zap.String("cron", pm.config.Cron))
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
				logger.Info("power_monitor_stopped")
				return
			}
		}
	} else {
		// Cron-only mode: block until stop signal
		<-pm.stopCh
		logger.Info("power_monitor_stopped")
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
			logger.Error("power_monitor_scan_panic_recovered", zap.Any("panic", r))
		}
	}()
	pm.scan()
}

// scan checks all board/PON combinations for abnormal RX power
func (pm *PowerMonitor) scan() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	logger.Debug("power_monitor_scanning_all_pons")

	// Scan the configured GPON slots/PONs. Falls back to the legacy C320 grid
	// ({1,2} x 16) when not configured, so existing setups are unaffected.
	boards := pm.config.Boards
	if len(boards) == 0 {
		boards = []int{1, 2}
	}
	ponsPerBoard := pm.config.PonsPerBoard
	if ponsPerBoard < 1 {
		ponsPerBoard = 16
	}

	alertCount := 0
	for _, boardID := range boards {
		for ponID := 1; ponID <= ponsPerBoard; ponID++ {
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
		logger.Warn("power_monitor_abnormal_rx_power_detected", zap.Int("alerts", alertCount))
	} else {
		logger.Debug("power_monitor_scan_complete_all_normal")
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

	logger.Warn("rx_power_alert",
		zap.Int("board", onu.Board),
		zap.Int("pon", onu.PON),
		zap.Int("onu_id", onu.ID),
		zap.String("rx_power", onu.RXPower),
		zap.String("event_type", eventType),
		zap.String("name", onu.Name))

	if pm.batcher != nil {
		pm.batcher.Add(event)
	} else if wc := resolveWebhook(pm.webhook); wc != nil {
		go wc.Send(event)
	}
}
