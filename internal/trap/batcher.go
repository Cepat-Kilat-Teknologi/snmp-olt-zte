package trap

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/model"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/pkg/logger"
	"go.uber.org/zap"
)

// Batcher collects trap events and flushes them periodically per severity,
// with deduplication by board/pon/onu within each batch window.
type batcherEntry struct {
	event        model.TrapEvent
	lastNotified time.Time
}

type Batcher struct {
	webhook         *WebhookClient
	fetcher         ONUDetailFetcher
	intervals       map[Severity]time.Duration
	RepeatIntervals map[Severity]time.Duration
	HighThreshold   float64
	LowThreshold    float64
	mu              sync.Mutex
	groups          map[Severity]map[string]*batcherEntry
	stopCh          chan struct{}
	closeOnce       sync.Once
}

// NewBatcher creates a new event batcher with per-severity flush intervals.
// fetcher is used to re-verify ONU status at flush time (pass nil to skip verification).
func NewBatcher(webhook *WebhookClient, fetcher ONUDetailFetcher, intervals map[Severity]time.Duration) *Batcher {
	return &Batcher{
		webhook:         webhook,
		fetcher:         fetcher,
		intervals:       intervals,
		RepeatIntervals: make(map[Severity]time.Duration),
		groups:          make(map[Severity]map[string]*batcherEntry),
		stopCh:          make(chan struct{}),
	}
}

func onuKey(event model.TrapEvent) string {
	return fmt.Sprintf("%d-%d-%d", event.Board, event.PON, event.OnuID)
}

// Add queues an event for batched sending, deduplicating by board/pon/onu.
// An ONU can only exist in one severity queue — adding removes from others.
func (b *Batcher) Add(event model.TrapEvent) {
	sev := eventSeverity(event.EventType)
	key := onuKey(event)

	b.mu.Lock()
	for s := range b.groups {
		if s != sev {
			delete(b.groups[s], key)
		}
	}
	if b.groups[sev] == nil {
		b.groups[sev] = make(map[string]*batcherEntry)
	}
	if existing, ok := b.groups[sev][key]; ok {
		existing.event = event
	} else {
		b.groups[sev][key] = &batcherEntry{event: event}
	}
	count := len(b.groups[sev])
	b.mu.Unlock()

	logger.Debug("batcher_event_queued",
		zap.String("event_type", event.EventType),
		zap.String("severity", severityLabel(sev)),
		zap.String("onu_key", key),
		zap.Int("queue_size", count))
}

// Remove deletes an ONU from all severity queues (e.g. when ONU comes back online).
func (b *Batcher) Remove(event model.TrapEvent) {
	key := onuKey(event)
	b.mu.Lock()
	for sev := range b.groups {
		if _, ok := b.groups[sev][key]; ok {
			delete(b.groups[sev], key)
			logger.Debug("batcher_event_removed",
				zap.String("onu_key", key),
				zap.String("severity", severityLabel(sev)))
		}
	}
	b.mu.Unlock()
}

// Start begins per-severity flush loops (blocking).
func (b *Batcher) Start() {
	var wg sync.WaitGroup

	for sev, interval := range b.intervals {
		if interval <= 0 {
			continue
		}
		wg.Add(1)
		go func(s Severity, d time.Duration) {
			defer wg.Done()
			logger.Info("batcher_severity_timer_started",
				zap.String("severity", severityLabel(s)),
				zap.Duration("interval", d))

			ticker := time.NewTicker(d)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					b.flushSeverity(s)
				case <-b.stopCh:
					b.flushSeverity(s)
					return
				}
			}
		}(sev, interval)
	}

	wg.Wait()
	logger.Info("batcher_stopped")
}

// Close stops the batcher and flushes remaining events.
func (b *Batcher) Close() error {
	b.closeOnce.Do(func() {
		close(b.stopCh)
	})
	return nil
}

// flushSeverity sends all queued events for a specific severity.
// Entries are kept in queue for repeat notifications if RepeatIntervals is set.
func (b *Batcher) flushSeverity(sev Severity) {
	b.mu.Lock()
	entries, ok := b.groups[sev]
	if !ok || len(entries) == 0 {
		b.mu.Unlock()
		return
	}

	// Collect keys to process (snapshot under lock)
	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	b.mu.Unlock()

	repeatInterval := b.RepeatIntervals[sev]
	var toRemove []string
	list := make([]model.TrapEvent, 0, len(keys))
	var notifiedKeys []string

	for _, key := range keys {
		b.mu.Lock()
		entry, ok := entries[key]
		if !ok {
			b.mu.Unlock()
			continue
		}
		e := entry.event
		lastNotified := entry.lastNotified
		b.mu.Unlock()

		// Skip if already notified and repeat interval hasn't passed
		if !lastNotified.IsZero() {
			if repeatInterval <= 0 {
				continue
			}
			if time.Since(lastNotified) < repeatInterval {
				continue
			}
		}

		// Re-verify ONU status via SNMP
		if b.fetcher != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_ = b.fetcher.InvalidateONUCache(ctx, e.Board, e.PON, e.OnuID)
			detail, err := b.fetcher.GetByBoardIDPonIDAndOnuID(ctx, e.Board, e.PON, e.OnuID)
			cancel()
			if err != nil || detail.ID == 0 {
				toRemove = append(toRemove, key)
				continue
			}
			if sev == SeverityMedium {
				rxPower, err := strconv.ParseFloat(detail.RXPower, 64)
				if err != nil || (rxPower >= b.LowThreshold && rxPower <= b.HighThreshold) {
					logger.Info("batcher_rx_power_normalized_skipping",
						zap.String("name", detail.Name),
						zap.String("rx_power", detail.RXPower),
						zap.Int("board", e.Board), zap.Int("pon", e.PON), zap.Int("onu_id", e.OnuID))
					toRemove = append(toRemove, key)
					continue
				}
				if rxPower > b.HighThreshold {
					e.EventType = "HighRxPower"
				} else {
					e.EventType = "LowRxPower"
				}
				e.Status = detail.Status
			} else {
				eventType := resolveEventType(detail.Status, detail.LastOfflineReason)
				if !alertEventTypes[eventType] {
					logger.Info("batcher_onu_recovered_skipping",
						zap.String("name", detail.Name),
						zap.String("verified_status", detail.Status),
						zap.Int("board", e.Board), zap.Int("pon", e.PON), zap.Int("onu_id", e.OnuID))
					toRemove = append(toRemove, key)
					continue
				}
				if eventSeverity(eventType) != sev {
					logger.Info("batcher_severity_changed_skipping",
						zap.String("name", detail.Name),
						zap.String("event_type", eventType),
						zap.String("old_severity", severityLabel(sev)),
						zap.String("new_severity", severityLabel(eventSeverity(eventType))))
					toRemove = append(toRemove, key)
					continue
				}
				e.EventType = eventType
				e.Status = detail.Status
			}
			e.RXPower = detail.RXPower
			e.LastOffline = detail.LastOffline
			e.LastOnline = detail.LastOnline
			if detail.Name != "" {
				e.Name = detail.Name
			}
			if detail.Description != "" {
				e.Description = detail.Description
			}
			e.SerialNumber = detail.SerialNumber
		}
		list = append(list, e)
		notifiedKeys = append(notifiedKeys, key)
	}

	// Clean up recovered/changed ONUs
	b.mu.Lock()
	for _, key := range toRemove {
		delete(entries, key)
	}
	b.mu.Unlock()

	if len(list) == 0 {
		logger.Info("batcher_all_recovered_nothing_to_send",
			zap.String("severity", severityLabel(sev)))
		return
	}

	// Mark notified entries with timestamp
	b.mu.Lock()
	now := time.Now()
	for _, key := range notifiedKeys {
		if entry, ok := entries[key]; ok {
			entry.lastNotified = now
			entry.event = list[0]
			for _, ev := range list {
				if onuKey(ev) == key {
					entry.event = ev
					break
				}
			}
		}
	}
	// If no repeat configured, remove after send
	if repeatInterval <= 0 {
		for _, key := range notifiedKeys {
			delete(entries, key)
		}
	}
	b.mu.Unlock()

	logger.Info("batcher_flushing",
		zap.String("severity", severityLabel(sev)),
		zap.Int("count", len(list)))

	payload, err := b.webhook.formatter.FormatBatch(sev, list)
	if err != nil {
		logger.Error("batcher_format_failed",
			zap.Error(err),
			zap.String("severity", severityLabel(sev)))
		return
	}

	b.webhook.sendPayload(payload)
}
