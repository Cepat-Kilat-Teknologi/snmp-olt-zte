package trap

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/model"
)

// mockONUListFetcher implements ONUListFetcher for testing
type mockONUListFetcher struct {
	onus map[string][]model.ONUInfoPerBoard
}

func (m *mockONUListFetcher) GetByBoardIDAndPonID(_ context.Context, boardID, ponID int) ([]model.ONUInfoPerBoard, error) {
	key := fmt.Sprintf("%d-%d", boardID, ponID)
	if onus, ok := m.onus[key]; ok {
		return onus, nil
	}
	return nil, fmt.Errorf("no data for board %d pon %d", boardID, ponID)
}

func TestPowerMonitor_HighRxPower(t *testing.T) {
	var webhookCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webhookCalls.Add(1)
		w.WriteHeader(200)
	}))
	defer server.Close()

	fetcher := &mockONUListFetcher{
		onus: map[string][]model.ONUInfoPerBoard{
			"1-1": {
				{Board: 1, PON: 1, ID: 1, Name: "Normal", RXPower: "-20.00", Status: "Online"},
				{Board: 1, PON: 1, ID: 2, Name: "Overload", RXPower: "-5.00", Status: "Online"},
			},
		},
	}

	webhook := NewWebhookClient(server.URL, 0, 5)
	pm := NewPowerMonitor(PowerMonitorConfig{
		Interval:      1 * time.Second,
		HighThreshold: -8.0,
		LowThreshold:  -25.0,
		Source:        "test",
	}, fetcher, webhook)

	// Run single scan
	pm.scan()

	time.Sleep(200 * time.Millisecond)

	if webhookCalls.Load() != 1 {
		t.Errorf("Expected 1 webhook call for high power ONU, got %d", webhookCalls.Load())
	}
}

func TestPowerMonitor_LowRxPower(t *testing.T) {
	var webhookCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webhookCalls.Add(1)
		w.WriteHeader(200)
	}))
	defer server.Close()

	fetcher := &mockONUListFetcher{
		onus: map[string][]model.ONUInfoPerBoard{
			"1-1": {
				{Board: 1, PON: 1, ID: 1, Name: "Normal", RXPower: "-20.00", Status: "Online"},
				{Board: 1, PON: 1, ID: 2, Name: "Weak", RXPower: "-27.50", Status: "Online"},
			},
		},
	}

	webhook := NewWebhookClient(server.URL, 0, 5)
	pm := NewPowerMonitor(PowerMonitorConfig{
		Interval:      1 * time.Second,
		HighThreshold: -8.0,
		LowThreshold:  -25.0,
		Source:        "test",
	}, fetcher, webhook)

	pm.scan()

	time.Sleep(200 * time.Millisecond)

	if webhookCalls.Load() != 1 {
		t.Errorf("Expected 1 webhook call for low power ONU, got %d", webhookCalls.Load())
	}
}

func TestPowerMonitor_NormalPower_NoAlert(t *testing.T) {
	var webhookCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webhookCalls.Add(1)
		w.WriteHeader(200)
	}))
	defer server.Close()

	fetcher := &mockONUListFetcher{
		onus: map[string][]model.ONUInfoPerBoard{
			"1-1": {
				{Board: 1, PON: 1, ID: 1, Name: "OK", RXPower: "-20.00", Status: "Online"},
				{Board: 1, PON: 1, ID: 2, Name: "OK2", RXPower: "-15.50", Status: "Online"},
			},
		},
	}

	webhook := NewWebhookClient(server.URL, 0, 5)
	pm := NewPowerMonitor(PowerMonitorConfig{
		Interval:      1 * time.Second,
		HighThreshold: -8.0,
		LowThreshold:  -25.0,
		Source:        "test",
	}, fetcher, webhook)

	pm.scan()

	time.Sleep(200 * time.Millisecond)

	if webhookCalls.Load() != 0 {
		t.Errorf("Expected 0 webhook calls for normal power, got %d", webhookCalls.Load())
	}
}

func TestPowerMonitor_DuplicateAlertSuppressed(t *testing.T) {
	var webhookCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webhookCalls.Add(1)
		w.WriteHeader(200)
	}))
	defer server.Close()

	fetcher := &mockONUListFetcher{
		onus: map[string][]model.ONUInfoPerBoard{
			"1-1": {
				{Board: 1, PON: 1, ID: 1, Name: "Overload", RXPower: "-5.00", Status: "Online"},
			},
		},
	}

	webhook := NewWebhookClient(server.URL, 0, 5)
	pm := NewPowerMonitor(PowerMonitorConfig{
		Interval:      1 * time.Second,
		HighThreshold: -8.0,
		LowThreshold:  -25.0,
		Source:        "test",
	}, fetcher, webhook)

	// Scan twice — second should be suppressed (30 min cooldown)
	pm.scan()
	time.Sleep(100 * time.Millisecond)
	pm.scan()
	time.Sleep(200 * time.Millisecond)

	if webhookCalls.Load() != 1 {
		t.Errorf("Expected 1 webhook (duplicate suppressed), got %d", webhookCalls.Load())
	}
}

func TestPowerMonitor_EmptyRxPower_Skipped(t *testing.T) {
	var webhookCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webhookCalls.Add(1)
		w.WriteHeader(200)
	}))
	defer server.Close()

	fetcher := &mockONUListFetcher{
		onus: map[string][]model.ONUInfoPerBoard{
			"1-1": {
				{Board: 1, PON: 1, ID: 1, Name: "NoData", RXPower: "", Status: "Offline"},
			},
		},
	}

	webhook := NewWebhookClient(server.URL, 0, 5)
	pm := NewPowerMonitor(PowerMonitorConfig{
		Interval:      1 * time.Second,
		HighThreshold: -8.0,
		LowThreshold:  -25.0,
		Source:        "test",
	}, fetcher, webhook)

	pm.scan()
	time.Sleep(200 * time.Millisecond)

	if webhookCalls.Load() != 0 {
		t.Errorf("Expected 0 webhook calls for empty rx_power, got %d", webhookCalls.Load())
	}
}

func TestPowerMonitor_NilWebhook_NoPanic(t *testing.T) {
	fetcher := &mockONUListFetcher{
		onus: map[string][]model.ONUInfoPerBoard{
			"1-1": {
				{Board: 1, PON: 1, ID: 1, Name: "Overload", RXPower: "-5.00", Status: "Online"},
			},
		},
	}

	pm := NewPowerMonitor(PowerMonitorConfig{
		Interval:      1 * time.Second,
		HighThreshold: -8.0,
		LowThreshold:  -25.0,
		Source:        "test",
	}, fetcher, nil)

	// Should not panic with nil webhook
	pm.scan()
}

func TestPowerMonitor_Close(t *testing.T) {
	fetcher := &mockONUListFetcher{onus: map[string][]model.ONUInfoPerBoard{}}
	pm := NewPowerMonitor(PowerMonitorConfig{
		Interval:      100 * time.Millisecond,
		HighThreshold: -8.0,
		LowThreshold:  -25.0,
	}, fetcher, nil)

	go pm.Start()
	time.Sleep(300 * time.Millisecond)
	err := pm.Close()
	if err != nil {
		t.Errorf("Expected no error on close, got %v", err)
	}
}

func TestPowerMonitor_InvalidRxPower_Skipped(t *testing.T) {
	var webhookCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webhookCalls.Add(1)
		w.WriteHeader(200)
	}))
	defer server.Close()

	fetcher := &mockONUListFetcher{
		onus: map[string][]model.ONUInfoPerBoard{
			"1-1": {
				{Board: 1, PON: 1, ID: 1, Name: "BadData", RXPower: "not-a-number", Status: "Online"},
			},
		},
	}

	webhook := NewWebhookClient(server.URL, 0, 5)
	pm := NewPowerMonitor(PowerMonitorConfig{
		Interval:      1 * time.Second,
		HighThreshold: -8.0,
		LowThreshold:  -25.0,
		Source:        "test",
	}, fetcher, webhook)

	pm.scan()
	time.Sleep(200 * time.Millisecond)

	if webhookCalls.Load() != 0 {
		t.Errorf("Expected 0 webhook calls for invalid rx_power, got %d", webhookCalls.Load())
	}
}

func TestPowerMonitor_SafeScan_RecoversPanic(t *testing.T) {
	// Create a power monitor with a fetcher that panics
	fetcher := &panicFetcher{}

	pm := NewPowerMonitor(PowerMonitorConfig{
		Interval:      1 * time.Second,
		HighThreshold: -8.0,
		LowThreshold:  -25.0,
		Source:        "test",
	}, fetcher, nil)

	// safeScan should recover from panic without crashing
	pm.safeScan() // Should not panic
}

// panicFetcher implements ONUListFetcher and panics on GetByBoardIDAndPonID
type panicFetcher struct{}

func (p *panicFetcher) GetByBoardIDAndPonID(_ context.Context, _, _ int) ([]model.ONUInfoPerBoard, error) {
	panic("simulated panic in fetcher")
}

func TestPowerMonitor_NormalPower_ClearsAlert(t *testing.T) {
	var webhookCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webhookCalls.Add(1)
		w.WriteHeader(200)
	}))
	defer server.Close()

	webhook := NewWebhookClient(server.URL, 0, 5)

	// First scan: ONU has high power -> triggers alert
	fetcher := &mockONUListFetcher{
		onus: map[string][]model.ONUInfoPerBoard{
			"1-1": {
				{Board: 1, PON: 1, ID: 1, Name: "ONU1", RXPower: "-5.00", Status: "Online"},
			},
		},
	}

	pm := NewPowerMonitor(PowerMonitorConfig{
		Interval:      1 * time.Second,
		HighThreshold: -8.0,
		LowThreshold:  -25.0,
		Source:        "test",
	}, fetcher, webhook)

	pm.scan()
	time.Sleep(200 * time.Millisecond)

	// Verify alert was set
	if _, exists := pm.alerted["1-1-1"]; !exists {
		t.Error("Expected alert state to be set for 1-1-1")
	}

	// Now change the ONU to normal power
	fetcher.onus["1-1"] = []model.ONUInfoPerBoard{
		{Board: 1, PON: 1, ID: 1, Name: "ONU1", RXPower: "-15.00", Status: "Online"},
	}

	// Manually clear alert time so it can re-alert if needed
	pm.alerted["1-1-1"] = time.Time{}

	pm.scan()
	time.Sleep(200 * time.Millisecond)

	// After normal power scan, the alert should be cleared
	if _, exists := pm.alerted["1-1-1"]; exists {
		t.Error("Expected alert state to be cleared after normal power")
	}
}

func TestShouldAlert(t *testing.T) {
	pm := &PowerMonitor{
		alerted: make(map[string]time.Time),
	}

	// First alert should pass
	if !pm.shouldAlert("1-1-1") {
		t.Error("Expected first alert to pass")
	}

	// Same key within 30 min should be suppressed
	if pm.shouldAlert("1-1-1") {
		t.Error("Expected duplicate alert to be suppressed")
	}

	// Different key should pass
	if !pm.shouldAlert("1-1-2") {
		t.Error("Expected different key to pass")
	}
}

func TestPowerMonitor_CronOnly(t *testing.T) {
	fetcher := &mockONUListFetcher{
		onus: map[string][]model.ONUInfoPerBoard{
			"1-1": {
				{Board: 1, PON: 1, ID: 1, Name: "ONU1", RXPower: "-15.00", Status: "Online"},
			},
		},
	}

	pm := NewPowerMonitor(PowerMonitorConfig{
		Interval:      0,
		Cron:          "* * * * *",
		Timezone:      "",
		HighThreshold: -8.0,
		LowThreshold:  -25.0,
		Source:        "test",
	}, fetcher, nil)

	if pm.cronRunner == nil {
		t.Error("Expected cronRunner to be set when Cron is configured")
	}

	// Start in cron-only mode (should block until Close)
	go pm.Start()
	time.Sleep(200 * time.Millisecond)

	pm.safeScan()

	err := pm.Close()
	if err != nil {
		t.Errorf("Expected no error on close, got %v", err)
	}
}

func TestPowerMonitor_IntervalAndCron(t *testing.T) {
	fetcher := &mockONUListFetcher{
		onus: map[string][]model.ONUInfoPerBoard{},
	}

	pm := NewPowerMonitor(PowerMonitorConfig{
		Interval:      100 * time.Millisecond,
		Cron:          "* * * * *",
		Timezone:      "Asia/Jakarta",
		HighThreshold: -8.0,
		LowThreshold:  -25.0,
		Source:        "test",
	}, fetcher, nil)

	if pm.cronRunner == nil {
		t.Error("Expected cronRunner to be set")
	}

	go pm.Start()
	time.Sleep(300 * time.Millisecond)
	_ = pm.Close()
}

func TestPowerMonitor_IntervalOnly_BackwardCompat(t *testing.T) {
	fetcher := &mockONUListFetcher{
		onus: map[string][]model.ONUInfoPerBoard{},
	}

	pm := NewPowerMonitor(PowerMonitorConfig{
		Interval:      100 * time.Millisecond,
		Cron:          "",
		HighThreshold: -8.0,
		LowThreshold:  -25.0,
		Source:        "test",
	}, fetcher, nil)

	if pm.cronRunner != nil {
		t.Error("Expected cronRunner to be nil when no cron configured")
	}

	go pm.Start()
	time.Sleep(300 * time.Millisecond)
	_ = pm.Close()
}

func TestPowerMonitor_BothDisabled(t *testing.T) {
	fetcher := &mockONUListFetcher{
		onus: map[string][]model.ONUInfoPerBoard{},
	}

	pm := NewPowerMonitor(PowerMonitorConfig{
		Interval:      0,
		Cron:          "",
		HighThreshold: -8.0,
		LowThreshold:  -25.0,
		Source:        "test",
	}, fetcher, nil)

	done := make(chan struct{})
	go func() {
		pm.Start()
		close(done)
	}()

	select {
	case <-done:
		// OK
	case <-time.After(2 * time.Second):
		t.Error("Expected Start to return immediately when both disabled")
		_ = pm.Close()
	}
}

func TestPowerMonitor_InvalidCron(t *testing.T) {
	fetcher := &mockONUListFetcher{
		onus: map[string][]model.ONUInfoPerBoard{},
	}

	pm := NewPowerMonitor(PowerMonitorConfig{
		Interval:      0,
		Cron:          "invalid cron expression",
		HighThreshold: -8.0,
		LowThreshold:  -25.0,
		Source:        "test",
	}, fetcher, nil)

	if pm.cronRunner != nil {
		t.Error("Expected cronRunner to be nil for invalid cron expression")
	}
}

func TestPowerMonitor_InvalidTimezone(t *testing.T) {
	fetcher := &mockONUListFetcher{
		onus: map[string][]model.ONUInfoPerBoard{},
	}

	pm := NewPowerMonitor(PowerMonitorConfig{
		Interval:      0,
		Cron:          "0 8 * * *",
		Timezone:      "Invalid/Timezone",
		HighThreshold: -8.0,
		LowThreshold:  -25.0,
		Source:        "test",
	}, fetcher, nil)

	if pm.cronRunner == nil {
		t.Error("Expected cronRunner to be set even with invalid timezone (fallback to local)")
	}

	pm.Close()
}

func TestPowerMonitor_CronWithTimezone(t *testing.T) {
	fetcher := &mockONUListFetcher{
		onus: map[string][]model.ONUInfoPerBoard{},
	}

	pm := NewPowerMonitor(PowerMonitorConfig{
		Interval:      0,
		Cron:          "0 8,12,15,17,0 * * *",
		Timezone:      "Asia/Jakarta",
		HighThreshold: -8.0,
		LowThreshold:  -25.0,
		Source:        "test",
	}, fetcher, nil)

	if pm.cronRunner == nil {
		t.Error("Expected cronRunner to be set")
	}

	pm.Close()
}
