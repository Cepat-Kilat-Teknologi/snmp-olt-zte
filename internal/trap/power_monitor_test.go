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
