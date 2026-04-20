package trap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/model"
)

func TestNewBatcher(t *testing.T) {
	webhook := NewWebhookClient("http://example.com", 0, 5, nil)
	intervals := map[Severity]time.Duration{
		SeverityCritical: 100 * time.Millisecond,
	}
	b := NewBatcher(webhook, nil, intervals)
	if b == nil {
		t.Fatal("expected non-nil Batcher")
	}
	if b.webhook != webhook {
		t.Error("expected webhook to be set")
	}
}

func TestOnuKey(t *testing.T) {
	event := model.TrapEvent{Board: 1, PON: 5, OnuID: 23}
	if got := onuKey(event); got != "1-5-23" {
		t.Errorf("onuKey() = %q, want 1-5-23", got)
	}
}

func TestBatcher_Add_Dedup(t *testing.T) {
	webhook := NewWebhookClient("http://example.com", 0, 5, nil)
	intervals := map[Severity]time.Duration{
		SeverityCritical: 1 * time.Hour,
	}
	b := NewBatcher(webhook, nil, intervals)

	e1 := model.TrapEvent{Board: 1, PON: 5, OnuID: 23, EventType: "LOS", Name: "First"}
	e2 := model.TrapEvent{Board: 1, PON: 5, OnuID: 23, EventType: "LOS", Name: "Updated"}
	e3 := model.TrapEvent{Board: 1, PON: 5, OnuID: 24, EventType: "LOS", Name: "Different"}

	b.Add(e1)
	b.Add(e2)
	b.Add(e3)

	b.mu.Lock()
	group := b.groups[SeverityCritical]
	b.mu.Unlock()

	if len(group) != 2 {
		t.Fatalf("expected 2 unique ONUs, got %d", len(group))
	}
	if group["1-5-23"].event.Name != "Updated" {
		t.Errorf("expected dedup to keep latest, got %q", group["1-5-23"].event.Name)
	}
}

func TestBatcher_Remove(t *testing.T) {
	webhook := NewWebhookClient("http://example.com", 0, 5, nil)
	b := NewBatcher(webhook, nil, map[Severity]time.Duration{SeverityCritical: time.Hour})

	b.Add(model.TrapEvent{Board: 1, PON: 1, OnuID: 1, EventType: "LOS", Name: "Customer-A"})
	b.Add(model.TrapEvent{Board: 1, PON: 1, OnuID: 2, EventType: "LOS", Name: "Customer-B"})

	b.Remove(model.TrapEvent{Board: 1, PON: 1, OnuID: 1})

	b.mu.Lock()
	group := b.groups[SeverityCritical]
	b.mu.Unlock()

	if len(group) != 1 {
		t.Fatalf("expected 1 after remove, got %d", len(group))
	}
	if _, ok := group["1-1-1"]; ok {
		t.Error("ONU 1-1-1 should have been removed")
	}
	if group["1-1-2"].event.Name != "Customer-B" {
		t.Error("ONU 1-1-2 should still exist")
	}
}

func TestBatcher_Remove_NotFound(t *testing.T) {
	webhook := NewWebhookClient("http://example.com", 0, 5, nil)
	b := NewBatcher(webhook, nil, map[Severity]time.Duration{})

	b.Remove(model.TrapEvent{Board: 1, PON: 1, OnuID: 99})
}

func TestBatcher_Add_MovesBetweenSeverities(t *testing.T) {
	webhook := NewWebhookClient("http://example.com", 0, 5, nil)
	b := NewBatcher(webhook, nil, map[Severity]time.Duration{
		SeverityCritical: time.Hour,
		SeverityLow:      time.Hour,
	})

	b.Add(model.TrapEvent{Board: 1, PON: 1, OnuID: 1, EventType: "DyingGasp", Name: "ONU-A"})

	b.mu.Lock()
	if len(b.groups[SeverityLow]) != 1 {
		t.Errorf("expected 1 in LOW, got %d", len(b.groups[SeverityLow]))
	}
	b.mu.Unlock()

	b.Add(model.TrapEvent{Board: 1, PON: 1, OnuID: 1, EventType: "LOS", Name: "ONU-A"})

	b.mu.Lock()
	lowCount := len(b.groups[SeverityLow])
	critCount := len(b.groups[SeverityCritical])
	b.mu.Unlock()

	if lowCount != 0 {
		t.Errorf("expected 0 in LOW after severity change, got %d", lowCount)
	}
	if critCount != 1 {
		t.Errorf("expected 1 in CRITICAL after severity change, got %d", critCount)
	}
}

func TestBatcher_FlushSeverity_SeverityChanged_Skipped(t *testing.T) {
	var webhookCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		webhookCalls.Add(1)
		w.WriteHeader(204)
	}))
	defer server.Close()

	fetcher := &mockONUDetailFetcher{
		result: model.ONUCustomerInfo{
			ID: 1, Name: "Changed", Status: "LOS",
		},
	}

	webhook := NewWebhookClient(server.URL, 0, 5, &GenericFormatter{})
	b := NewBatcher(webhook, fetcher, map[Severity]time.Duration{SeverityLow: time.Hour})

	b.Add(model.TrapEvent{Board: 1, PON: 1, OnuID: 1, EventType: "DyingGasp"})
	b.flushSeverity(SeverityLow)

	time.Sleep(200 * time.Millisecond)

	if webhookCalls.Load() != 0 {
		t.Errorf("expected 0 webhook (severity changed from LOW to CRITICAL), got %d", webhookCalls.Load())
	}
}

func TestBatcher_FlushSeverity(t *testing.T) {
	var webhookCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		webhookCalls.Add(1)
		w.WriteHeader(204)
	}))
	defer server.Close()

	webhook := NewWebhookClient(server.URL, 0, 5, &GenericFormatter{})
	intervals := map[Severity]time.Duration{
		SeverityCritical: 1 * time.Hour,
	}
	b := NewBatcher(webhook, nil, intervals)

	b.Add(model.TrapEvent{Board: 1, PON: 1, OnuID: 1, EventType: "LOS"})
	b.Add(model.TrapEvent{Board: 1, PON: 1, OnuID: 2, EventType: "LOS"})

	b.flushSeverity(SeverityCritical)

	time.Sleep(200 * time.Millisecond)

	if webhookCalls.Load() != 1 {
		t.Errorf("expected 1 webhook call (batched), got %d", webhookCalls.Load())
	}

	b.mu.Lock()
	remaining := len(b.groups[SeverityCritical])
	b.mu.Unlock()
	if remaining != 0 {
		t.Errorf("expected 0 remaining events, got %d", remaining)
	}
}

func TestBatcher_FlushSeverity_Empty(t *testing.T) {
	webhook := NewWebhookClient("http://example.com", 0, 5, &GenericFormatter{})
	b := NewBatcher(webhook, nil, map[Severity]time.Duration{})

	b.flushSeverity(SeverityCritical)
}

func TestBatcher_FlushSeverity_FormatError(t *testing.T) {
	webhook := NewWebhookClient("http://example.com", 0, 5, &failingFormatter{})
	b := NewBatcher(webhook, nil, map[Severity]time.Duration{SeverityCritical: time.Hour})

	b.Add(model.TrapEvent{Board: 1, PON: 1, OnuID: 1, EventType: "LOS"})
	b.flushSeverity(SeverityCritical)
}

func TestBatcher_StartAndClose(t *testing.T) {
	var webhookCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		webhookCalls.Add(1)
		w.WriteHeader(204)
	}))
	defer server.Close()

	webhook := NewWebhookClient(server.URL, 0, 5, &GenericFormatter{})
	intervals := map[Severity]time.Duration{
		SeverityCritical: 100 * time.Millisecond,
		SeverityLow:      100 * time.Millisecond,
	}
	b := NewBatcher(webhook, nil, intervals)

	b.Add(model.TrapEvent{Board: 1, PON: 1, OnuID: 1, EventType: "LOS"})
	b.Add(model.TrapEvent{Board: 1, PON: 1, OnuID: 2, EventType: "DyingGasp"})

	done := make(chan struct{})
	go func() {
		b.Start()
		close(done)
	}()

	time.Sleep(300 * time.Millisecond)
	_ = b.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("batcher did not stop")
	}

	if webhookCalls.Load() < 2 {
		t.Errorf("expected at least 2 webhook calls (2 severities), got %d", webhookCalls.Load())
	}
}

func TestBatcher_CloseFlushesRemaining(t *testing.T) {
	var webhookCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		webhookCalls.Add(1)
		w.WriteHeader(204)
	}))
	defer server.Close()

	webhook := NewWebhookClient(server.URL, 0, 5, &GenericFormatter{})
	intervals := map[Severity]time.Duration{
		SeverityCritical: 1 * time.Hour,
	}
	b := NewBatcher(webhook, nil, intervals)

	b.Add(model.TrapEvent{Board: 1, PON: 1, OnuID: 1, EventType: "LOS"})

	done := make(chan struct{})
	go func() {
		b.Start()
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	_ = b.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("batcher did not stop")
	}

	time.Sleep(200 * time.Millisecond)

	if webhookCalls.Load() != 1 {
		t.Errorf("expected 1 webhook call on close flush, got %d", webhookCalls.Load())
	}
}

func TestBatcher_FlushSeverity_WithFetcherVerification(t *testing.T) {
	var webhookCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		webhookCalls.Add(1)
		w.WriteHeader(204)
	}))
	defer server.Close()

	fetcher := &mockONUDetailFetcher{
		result: model.ONUCustomerInfo{
			ID: 1, Name: "Still-Offline", Status: "LOS",
			Description: "Alamat X", SerialNumber: "SN123", RXPower: "-20.00",
			LastOffline: "2026-04-20 19:00:00",
		},
	}

	webhook := NewWebhookClient(server.URL, 0, 5, &GenericFormatter{})
	b := NewBatcher(webhook, fetcher, map[Severity]time.Duration{SeverityCritical: time.Hour})

	b.Add(model.TrapEvent{Board: 1, PON: 1, OnuID: 1, EventType: "LOS"})
	b.flushSeverity(SeverityCritical)

	time.Sleep(200 * time.Millisecond)

	if webhookCalls.Load() != 1 {
		t.Errorf("expected 1 webhook for verified offline ONU, got %d", webhookCalls.Load())
	}
}

func TestBatcher_FlushSeverity_FetcherSaysOnline_Skipped(t *testing.T) {
	var webhookCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		webhookCalls.Add(1)
		w.WriteHeader(204)
	}))
	defer server.Close()

	fetcher := &mockONUDetailFetcher{
		result: model.ONUCustomerInfo{
			ID: 1, Name: "Recovered", Status: "Online",
		},
	}

	webhook := NewWebhookClient(server.URL, 0, 5, &GenericFormatter{})
	b := NewBatcher(webhook, fetcher, map[Severity]time.Duration{SeverityCritical: time.Hour})

	b.Add(model.TrapEvent{Board: 1, PON: 1, OnuID: 1, EventType: "LOS"})
	b.flushSeverity(SeverityCritical)

	time.Sleep(200 * time.Millisecond)

	if webhookCalls.Load() != 0 {
		t.Errorf("expected 0 webhook for recovered ONU, got %d", webhookCalls.Load())
	}
}

func TestBatcher_FlushSeverity_FetcherError_Skipped(t *testing.T) {
	fetcher := &mockONUDetailFetcher{
		err: context.DeadlineExceeded,
	}

	webhook := NewWebhookClient("http://example.com", 0, 5, &GenericFormatter{})
	b := NewBatcher(webhook, fetcher, map[Severity]time.Duration{SeverityCritical: time.Hour})

	b.Add(model.TrapEvent{Board: 1, PON: 1, OnuID: 1, EventType: "LOS"})
	b.flushSeverity(SeverityCritical)
}

func TestBatcher_FlushSeverity_FetcherZeroID_Skipped(t *testing.T) {
	fetcher := &mockONUDetailFetcher{
		result: model.ONUCustomerInfo{ID: 0},
	}

	webhook := NewWebhookClient("http://example.com", 0, 5, &GenericFormatter{})
	b := NewBatcher(webhook, fetcher, map[Severity]time.Duration{SeverityCritical: time.Hour})

	b.Add(model.TrapEvent{Board: 1, PON: 1, OnuID: 1, EventType: "LOS"})
	b.flushSeverity(SeverityCritical)
}

func TestBatcher_FlushSeverity_FetcherUpdatesEventData(t *testing.T) {
	fetcher := &mockONUDetailFetcher{
		result: model.ONUCustomerInfo{
			ID: 1, Name: "SNMP-Name", Description: "SNMP-Addr",
			Status: "Dying Gasp", SerialNumber: "SN999",
			RXPower: "-15.00", LastOffline: "2026-04-20 19:30:00",
		},
	}

	var webhookCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		webhookCalls.Add(1)
		w.WriteHeader(204)
	}))
	defer server.Close()

	webhook := NewWebhookClient(server.URL, 0, 5, &GenericFormatter{})
	b := NewBatcher(webhook, fetcher, map[Severity]time.Duration{SeverityLow: time.Hour})

	b.Add(model.TrapEvent{Board: 1, PON: 1, OnuID: 1, EventType: "DyingGasp", Name: "Old-Name"})
	b.flushSeverity(SeverityLow)

	time.Sleep(200 * time.Millisecond)

	if webhookCalls.Load() != 1 {
		t.Errorf("expected 1 webhook, got %d", webhookCalls.Load())
	}
}

func TestBatcher_FlushSeverity_Medium_StillAbnormal(t *testing.T) {
	var webhookCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		webhookCalls.Add(1)
		w.WriteHeader(204)
	}))
	defer server.Close()

	fetcher := &mockONUDetailFetcher{
		result: model.ONUCustomerInfo{
			ID: 1, Name: "Low-Power", Status: "Online",
			RXPower: "-30.00", SerialNumber: "SN123",
		},
	}

	webhook := NewWebhookClient(server.URL, 0, 5, &GenericFormatter{})
	b := NewBatcher(webhook, fetcher, map[Severity]time.Duration{SeverityMedium: time.Hour})
	b.HighThreshold = -8.0
	b.LowThreshold = -28.0

	b.Add(model.TrapEvent{Board: 1, PON: 1, OnuID: 1, EventType: "LowRxPower"})
	b.flushSeverity(SeverityMedium)
	time.Sleep(200 * time.Millisecond)

	if webhookCalls.Load() != 1 {
		t.Errorf("expected 1 webhook for still-abnormal RX power, got %d", webhookCalls.Load())
	}
}

func TestBatcher_FlushSeverity_Medium_HighRxPower(t *testing.T) {
	var webhookCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		webhookCalls.Add(1)
		w.WriteHeader(204)
	}))
	defer server.Close()

	fetcher := &mockONUDetailFetcher{
		result: model.ONUCustomerInfo{
			ID: 1, Name: "High-Power", Status: "Online",
			RXPower: "-5.00", SerialNumber: "SN456",
		},
	}

	webhook := NewWebhookClient(server.URL, 0, 5, &GenericFormatter{})
	b := NewBatcher(webhook, fetcher, map[Severity]time.Duration{SeverityMedium: time.Hour})
	b.HighThreshold = -8.0
	b.LowThreshold = -28.0

	b.Add(model.TrapEvent{Board: 1, PON: 1, OnuID: 1, EventType: "HighRxPower"})
	b.flushSeverity(SeverityMedium)
	time.Sleep(200 * time.Millisecond)

	if webhookCalls.Load() != 1 {
		t.Errorf("expected 1 webhook for high RX power, got %d", webhookCalls.Load())
	}
}

func TestBatcher_FlushSeverity_Medium_Normalized_Skipped(t *testing.T) {
	var webhookCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		webhookCalls.Add(1)
		w.WriteHeader(204)
	}))
	defer server.Close()

	fetcher := &mockONUDetailFetcher{
		result: model.ONUCustomerInfo{
			ID: 1, Name: "Normal-Now", Status: "Online",
			RXPower: "-18.00",
		},
	}

	webhook := NewWebhookClient(server.URL, 0, 5, &GenericFormatter{})
	b := NewBatcher(webhook, fetcher, map[Severity]time.Duration{SeverityMedium: time.Hour})
	b.HighThreshold = -8.0
	b.LowThreshold = -28.0

	b.Add(model.TrapEvent{Board: 1, PON: 1, OnuID: 1, EventType: "LowRxPower"})
	b.flushSeverity(SeverityMedium)
	time.Sleep(200 * time.Millisecond)

	if webhookCalls.Load() != 0 {
		t.Errorf("expected 0 webhook for normalized RX power, got %d", webhookCalls.Load())
	}
}

func TestBatcher_FlushSeverity_Medium_BadRxPower_Skipped(t *testing.T) {
	fetcher := &mockONUDetailFetcher{
		result: model.ONUCustomerInfo{
			ID: 1, Status: "Online", RXPower: "not-a-number",
		},
	}

	webhook := NewWebhookClient("http://example.com", 0, 5, &GenericFormatter{})
	b := NewBatcher(webhook, fetcher, map[Severity]time.Duration{SeverityMedium: time.Hour})
	b.HighThreshold = -8.0
	b.LowThreshold = -28.0

	b.Add(model.TrapEvent{Board: 1, PON: 1, OnuID: 1, EventType: "LowRxPower"})
	b.flushSeverity(SeverityMedium)
}

func TestPowerMonitor_SetBatcher(t *testing.T) {
	fetcher := &mockONUListFetcher{onus: map[string][]model.ONUInfoPerBoard{}}
	pm := NewPowerMonitor(PowerMonitorConfig{}, fetcher, nil)

	webhook := NewWebhookClient("http://example.com", 0, 5, &GenericFormatter{})
	b := NewBatcher(webhook, nil, map[Severity]time.Duration{})

	pm.SetBatcher(b)
	if pm.batcher != b {
		t.Error("expected batcher to be set")
	}
}

func TestPowerMonitor_SendAlert_UsesBatcher(t *testing.T) {
	webhook := NewWebhookClient("http://example.com", 0, 5, &GenericFormatter{})
	b := NewBatcher(webhook, nil, map[Severity]time.Duration{SeverityMedium: time.Hour})

	fetcher := &mockONUListFetcher{
		onus: map[string][]model.ONUInfoPerBoard{
			"1-1": {{Board: 1, PON: 1, ID: 1, Name: "Test", RXPower: "-5.00", Status: "Online"}},
		},
	}

	pm := NewPowerMonitor(PowerMonitorConfig{
		HighThreshold: -8.0, LowThreshold: -25.0, Source: "test",
	}, fetcher, nil)
	pm.SetBatcher(b)

	pm.scan()
	time.Sleep(100 * time.Millisecond)

	b.mu.Lock()
	count := len(b.groups[SeverityMedium])
	b.mu.Unlock()

	if count != 1 {
		t.Errorf("expected 1 event in MEDIUM batcher queue, got %d", count)
	}
}

func TestBatcher_RepeatInterval_SkipsRecentlyNotified(t *testing.T) {
	var webhookCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		webhookCalls.Add(1)
		w.WriteHeader(204)
	}))
	defer server.Close()

	fetcher := &mockONUDetailFetcher{
		result: model.ONUCustomerInfo{
			ID: 1, Name: "Still-LOS", Status: "LOS",
		},
	}

	webhook := NewWebhookClient(server.URL, 0, 5, &GenericFormatter{})
	b := NewBatcher(webhook, fetcher, map[Severity]time.Duration{SeverityCritical: 100 * time.Millisecond})
	b.RepeatIntervals[SeverityCritical] = 1 * time.Hour

	b.Add(model.TrapEvent{Board: 1, PON: 1, OnuID: 1, EventType: "LOS"})

	// First flush — should send
	b.flushSeverity(SeverityCritical)
	time.Sleep(200 * time.Millisecond)
	if webhookCalls.Load() != 1 {
		t.Fatalf("expected 1 webhook on first flush, got %d", webhookCalls.Load())
	}

	// Second flush immediately — should skip (repeat interval not passed)
	b.flushSeverity(SeverityCritical)
	time.Sleep(200 * time.Millisecond)
	if webhookCalls.Load() != 1 {
		t.Errorf("expected still 1 webhook (repeat skipped), got %d", webhookCalls.Load())
	}

	// Entry should still be in queue
	b.mu.Lock()
	count := len(b.groups[SeverityCritical])
	b.mu.Unlock()
	if count != 1 {
		t.Errorf("expected entry kept in queue for repeat, got %d", count)
	}
}

func TestBatcher_RepeatInterval_SendsAfterInterval(t *testing.T) {
	var webhookCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		webhookCalls.Add(1)
		w.WriteHeader(204)
	}))
	defer server.Close()

	fetcher := &mockONUDetailFetcher{
		result: model.ONUCustomerInfo{
			ID: 1, Name: "Still-LOS", Status: "LOS",
		},
	}

	webhook := NewWebhookClient(server.URL, 0, 5, &GenericFormatter{})
	b := NewBatcher(webhook, fetcher, map[Severity]time.Duration{SeverityCritical: 100 * time.Millisecond})
	b.RepeatIntervals[SeverityCritical] = 50 * time.Millisecond

	b.Add(model.TrapEvent{Board: 1, PON: 1, OnuID: 1, EventType: "LOS"})

	b.flushSeverity(SeverityCritical)
	time.Sleep(200 * time.Millisecond)
	if webhookCalls.Load() != 1 {
		t.Fatalf("expected 1 on first flush, got %d", webhookCalls.Load())
	}

	// Wait for repeat interval to pass
	time.Sleep(100 * time.Millisecond)

	b.flushSeverity(SeverityCritical)
	time.Sleep(200 * time.Millisecond)
	if webhookCalls.Load() != 2 {
		t.Errorf("expected 2 (repeat sent), got %d", webhookCalls.Load())
	}
}

func TestBatcher_RepeatInterval_ZeroRepeat_SkipsAlreadyNotified(t *testing.T) {
	fetcher := &mockONUDetailFetcher{
		result: model.ONUCustomerInfo{ID: 1, Name: "LOS", Status: "LOS"},
	}

	var webhookCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		webhookCalls.Add(1)
		w.WriteHeader(204)
	}))
	defer server.Close()

	webhook := NewWebhookClient(server.URL, 0, 5, &GenericFormatter{})
	b := NewBatcher(webhook, fetcher, map[Severity]time.Duration{SeverityCritical: 100 * time.Millisecond})
	// No repeat (default 0) but manually set lastNotified to simulate already notified + kept
	b.mu.Lock()
	b.groups[SeverityCritical] = map[string]*batcherEntry{
		"1-1-1": {event: model.TrapEvent{Board: 1, PON: 1, OnuID: 1, EventType: "LOS"}, lastNotified: time.Now()},
	}
	b.mu.Unlock()

	b.flushSeverity(SeverityCritical)
	time.Sleep(200 * time.Millisecond)

	if webhookCalls.Load() != 0 {
		t.Errorf("expected 0 webhook (already notified, no repeat), got %d", webhookCalls.Load())
	}
}

func TestBatcher_FlushSeverity_EntryRemovedDuringIteration(t *testing.T) {
	// Simulate entry removed between keys snapshot and per-key lookup
	// by using a fetcher whose InvalidateONUCache deletes the second entry
	deletingFetcher := &sideEffectFetcher{
		result: model.ONUCustomerInfo{ID: 1, Name: "LOS", Status: "LOS"},
	}

	var webhookCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		webhookCalls.Add(1)
		w.WriteHeader(204)
	}))
	defer server.Close()

	webhook := NewWebhookClient(server.URL, 0, 5, &GenericFormatter{})
	b := NewBatcher(webhook, deletingFetcher, map[Severity]time.Duration{SeverityCritical: time.Hour})
	deletingFetcher.batcher = b
	deletingFetcher.deleteKey = "1-1-2"

	b.Add(model.TrapEvent{Board: 1, PON: 1, OnuID: 1, EventType: "LOS"})
	b.Add(model.TrapEvent{Board: 1, PON: 1, OnuID: 2, EventType: "LOS"})

	b.flushSeverity(SeverityCritical)
	time.Sleep(200 * time.Millisecond)
}

type sideEffectFetcher struct {
	result    model.ONUCustomerInfo
	batcher   *Batcher
	deleteKey string
	called    bool
}

func (f *sideEffectFetcher) GetByBoardIDPonIDAndOnuID(_ context.Context, _, _, _ int) (model.ONUCustomerInfo, error) {
	if !f.called && f.batcher != nil && f.deleteKey != "" {
		f.called = true
		f.batcher.mu.Lock()
		delete(f.batcher.groups[SeverityCritical], f.deleteKey)
		f.batcher.mu.Unlock()
	}
	return f.result, nil
}

func (f *sideEffectFetcher) InvalidateONUCache(_ context.Context, _, _, _ int) error {
	return nil
}

func TestBatcher_NoRepeat_RemovesAfterSend(t *testing.T) {
	var webhookCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		webhookCalls.Add(1)
		w.WriteHeader(204)
	}))
	defer server.Close()

	fetcher := &mockONUDetailFetcher{
		result: model.ONUCustomerInfo{ID: 1, Name: "LOS", Status: "LOS"},
	}

	webhook := NewWebhookClient(server.URL, 0, 5, &GenericFormatter{})
	b := NewBatcher(webhook, fetcher, map[Severity]time.Duration{SeverityCritical: 100 * time.Millisecond})
	// No repeat interval set (default 0)

	b.Add(model.TrapEvent{Board: 1, PON: 1, OnuID: 1, EventType: "LOS"})
	b.flushSeverity(SeverityCritical)
	time.Sleep(200 * time.Millisecond)

	b.mu.Lock()
	count := len(b.groups[SeverityCritical])
	b.mu.Unlock()
	if count != 0 {
		t.Errorf("expected entry removed after send (no repeat), got %d", count)
	}
}

func TestBatcher_EmptyIntervals(t *testing.T) {
	webhook := NewWebhookClient("http://example.com", 0, 5, &GenericFormatter{})
	b := NewBatcher(webhook, nil, map[Severity]time.Duration{})

	done := make(chan struct{})
	go func() {
		b.Start()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("batcher with empty intervals should return immediately")
		_ = b.Close()
	}
}

func TestBatcher_ZeroIntervalSkipped(t *testing.T) {
	webhook := NewWebhookClient("http://example.com", 0, 5, &GenericFormatter{})
	intervals := map[Severity]time.Duration{
		SeverityCritical: 0,
	}
	b := NewBatcher(webhook, nil, intervals)

	done := make(chan struct{})
	go func() {
		b.Start()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("batcher with zero interval should return immediately")
		_ = b.Close()
	}
}
