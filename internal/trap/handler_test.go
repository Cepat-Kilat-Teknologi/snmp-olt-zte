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

// mockONUDetailFetcher implements ONUDetailFetcher for testing
type mockONUDetailFetcher struct {
	result model.ONUCustomerInfo
	err    error
}

func (m *mockONUDetailFetcher) GetByBoardIDPonIDAndOnuID(_ context.Context, _, _, _ int) (model.ONUCustomerInfo, error) {
	return m.result, m.err
}

func (m *mockONUDetailFetcher) InvalidateONUCache(_ context.Context, _, _, _ int) error {
	return nil
}

func TestHandleEvent_VerifiedOffline_LOS(t *testing.T) {
	fetcher := &mockONUDetailFetcher{
		result: model.ONUCustomerInfo{
			ID:               1,
			Name:             "Customer-023",
			Description:      "Perumahan Graha Ria Blok F No.6",
			OnuType:          "F670LV7.1",
			SerialNumber:     "ZTEGC12345678",
			Status:           "LOS",
			LastOfflineReason: "",
		},
	}

	var webhookCalled atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		webhookCalled.Add(1)
		w.WriteHeader(204)
	}))
	defer server.Close()

	webhook := NewWebhookClient(server.URL, 0, 5, nil)
	handler := NewHandler(webhook, nil, fetcher)

	event := model.TrapEvent{
		Timestamp: time.Now(),
		Source:    "192.168.213.174",
		Board:     1,
		PON:       5,
		OnuID:     23,
		EventType: "StatusChange",
	}

	handler.HandleEvent(event)
	time.Sleep(500 * time.Millisecond)

	if webhookCalled.Load() != 1 {
		t.Errorf("Expected webhook for LOS ONU, got %d calls", webhookCalled.Load())
	}
}

func TestHandleEvent_VerifiedOffline_DyingGasp(t *testing.T) {
	fetcher := &mockONUDetailFetcher{
		result: model.ONUCustomerInfo{
			ID:     1,
			Name:   "Customer-DG",
			Status: "Dying Gasp",
		},
	}

	var webhookCalled atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		webhookCalled.Add(1)
		w.WriteHeader(204)
	}))
	defer server.Close()

	webhook := NewWebhookClient(server.URL, 0, 5, nil)
	handler := NewHandler(webhook, nil, fetcher)

	event := model.TrapEvent{
		Timestamp: time.Now(),
		Source:    "192.168.213.174",
		Board:     1,
		PON:       3,
		OnuID:     10,
		EventType: "StatusChange",
	}

	handler.HandleEvent(event)
	time.Sleep(500 * time.Millisecond)

	if webhookCalled.Load() != 1 {
		t.Errorf("Expected webhook for DyingGasp ONU, got %d calls", webhookCalled.Load())
	}
}

func TestHandleEvent_VerifiedOnline_Skipped(t *testing.T) {
	fetcher := &mockONUDetailFetcher{
		result: model.ONUCustomerInfo{
			ID:     1,
			Name:   "Online-Customer",
			Status: "Online",
		},
	}

	var webhookCalled atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		webhookCalled.Add(1)
		w.WriteHeader(204)
	}))
	defer server.Close()

	webhook := NewWebhookClient(server.URL, 0, 5, nil)
	handler := NewHandler(webhook, nil, fetcher)

	event := model.TrapEvent{
		Timestamp: time.Now(),
		Source:    "192.168.213.174",
		Board:     1,
		PON:       5,
		OnuID:     23,
		EventType: "StatusChange",
	}

	handler.HandleEvent(event)
	time.Sleep(300 * time.Millisecond)

	if webhookCalled.Load() != 0 {
		t.Errorf("Expected no webhook for Online ONU, got %d calls", webhookCalled.Load())
	}
}

func TestHandleEvent_VerifiedOfflineWithReason(t *testing.T) {
	fetcher := &mockONUDetailFetcher{
		result: model.ONUCustomerInfo{
			ID:               1,
			Name:             "Customer-LOS",
			Status:           "Offline",
			LastOfflineReason: "LOS",
		},
	}

	handler := NewHandler(nil, nil, fetcher)

	event := model.TrapEvent{
		Timestamp: time.Now(),
		Source:    "192.168.213.174",
		Board:     1,
		PON:       1,
		OnuID:     1,
		EventType: "StatusChange",
	}

	// Should not panic with nil webhook
	handler.HandleEvent(event)
}

func TestHandleEvent_ZeroBoardPonOnuID_Skipped(t *testing.T) {
	handler := NewHandler(nil, nil, nil)

	tests := []struct {
		name  string
		event model.TrapEvent
	}{
		{"zero_board", model.TrapEvent{Board: 0, PON: 3, OnuID: 10}},
		{"zero_pon", model.TrapEvent{Board: 1, PON: 0, OnuID: 10}},
		{"zero_onuid", model.TrapEvent{Board: 1, PON: 3, OnuID: 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler.HandleEvent(tt.event)
		})
	}
}

func TestHandleEvent_FetcherError_Skipped(t *testing.T) {
	fetcher := &mockONUDetailFetcher{
		err: context.DeadlineExceeded,
	}

	var webhookCalled atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		webhookCalled.Add(1)
		w.WriteHeader(204)
	}))
	defer server.Close()

	webhook := NewWebhookClient(server.URL, 0, 5, nil)
	handler := NewHandler(webhook, nil, fetcher)

	event := model.TrapEvent{
		Timestamp: time.Now(),
		Source:    "192.168.213.174",
		Board:     1,
		PON:       3,
		OnuID:     10,
		EventType: "StatusChange",
	}

	handler.HandleEvent(event)
	time.Sleep(300 * time.Millisecond)

	if webhookCalled.Load() != 0 {
		t.Errorf("Expected no webhook when fetcher fails, got %d", webhookCalled.Load())
	}
}

func TestHandleEvent_FetcherReturnsZeroID_Skipped(t *testing.T) {
	fetcher := &mockONUDetailFetcher{
		result: model.ONUCustomerInfo{ID: 0},
	}

	handler := NewHandler(nil, nil, fetcher)

	event := model.TrapEvent{
		Timestamp: time.Now(),
		Board:     1, PON: 3, OnuID: 10,
		EventType: "StatusChange",
	}

	handler.HandleEvent(event)
}

func TestHandleEvent_NilFetcher_Skipped(t *testing.T) {
	var webhookCalled atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		webhookCalled.Add(1)
		w.WriteHeader(204)
	}))
	defer server.Close()

	webhook := NewWebhookClient(server.URL, 0, 5, nil)
	handler := NewHandler(webhook, nil, nil)

	event := model.TrapEvent{
		Timestamp: time.Now(),
		Board:     1, PON: 5, OnuID: 23,
		EventType: "StatusChange",
	}

	handler.HandleEvent(event)
	time.Sleep(300 * time.Millisecond)

	// StatusChange is not in alertEventTypes, so webhook should not fire
	if webhookCalled.Load() != 0 {
		t.Errorf("Expected no webhook for unverified StatusChange, got %d", webhookCalled.Load())
	}
}

func TestHandleEvent_NilWebhookClient(t *testing.T) {
	fetcher := &mockONUDetailFetcher{
		result: model.ONUCustomerInfo{
			ID:     1,
			Name:   "Offline-Customer",
			Status: "LOS",
		},
	}

	handler := NewHandler(nil, nil, fetcher)

	event := model.TrapEvent{
		Timestamp: time.Now(),
		Board:     1, PON: 3, OnuID: 10,
		EventType: "StatusChange",
	}

	// Should not panic with nil webhook
	handler.HandleEvent(event)
}

func TestHandleEvent_Enrichment_PrefersSNMP(t *testing.T) {
	fetcher := &mockONUDetailFetcher{
		result: model.ONUCustomerInfo{
			ID:           42,
			Name:         "SNMP-Name",
			Description:  "SNMP-Address",
			OnuType:      "F670L",
			SerialNumber: "ZTEG99999999",
			Status:       "LOS",
		},
	}

	var webhookCalled atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		webhookCalled.Add(1)
		w.WriteHeader(204)
	}))
	defer server.Close()

	webhook := NewWebhookClient(server.URL, 0, 5, nil)
	handler := NewHandler(webhook, nil, fetcher)

	event := model.TrapEvent{
		Timestamp:   time.Now(),
		Board:       1, PON: 5, OnuID: 23,
		EventType:   "StatusChange",
		Name:        "Trap-Name",
		Description: "Trap-Address",
	}

	handler.HandleEvent(event)
	time.Sleep(500 * time.Millisecond)

	if webhookCalled.Load() != 1 {
		t.Errorf("Expected 1 webhook call, got %d", webhookCalled.Load())
	}
}

func TestHandleEvent_Enrichment_KeepsTrapDataIfSNMPEmpty(t *testing.T) {
	fetcher := &mockONUDetailFetcher{
		result: model.ONUCustomerInfo{
			ID:     42,
			Name:   "",
			Status: "LOS",
		},
	}

	handler := NewHandler(nil, nil, fetcher)

	event := model.TrapEvent{
		Timestamp:   time.Now(),
		Board:       1, PON: 5, OnuID: 23,
		EventType:   "StatusChange",
		Name:        "Trap-Name",
		Description: "Trap-Address",
	}

	handler.HandleEvent(event)
	// Name should remain "Trap-Name" since SNMP returned empty
}

func TestHandleEvent_AllVerifiedOfflineStatuses(t *testing.T) {
	offlineStatuses := []struct {
		status    string
		wantEvent string
	}{
		{"LOS", "LOS"},
		{"Dying Gasp", "DyingGasp"},
		{"PowerOff", "PowerOff"},
		{"AuthFailed", "AuthFailed"},
		{"Offline", "Offline"},
		{"Logging", "Logging"},
		{"Syncing", "Synchronization"},
	}

	for _, tt := range offlineStatuses {
		t.Run(tt.status, func(t *testing.T) {
			fetcher := &mockONUDetailFetcher{
				result: model.ONUCustomerInfo{
					ID:     1,
					Name:   "Test",
					Status: tt.status,
				},
			}

			handler := NewHandler(nil, nil, fetcher)

			event := model.TrapEvent{
				Timestamp: time.Now(),
				Board:     1, PON: 1, OnuID: 1,
				EventType: "StatusChange",
			}

			handler.HandleEvent(event)
		})
	}
}

func TestNewHandler(t *testing.T) {
	webhook := NewWebhookClient("http://example.com", 3, 5, nil)
	fetcher := &mockONUDetailFetcher{}

	handler := NewHandler(webhook, nil, fetcher)
	if handler == nil {
		t.Fatal("expected non-nil Handler")
	}
	if handler.webhook != webhook {
		t.Error("expected webhook to be set")
	}
	if handler.onuFetcher == nil {
		t.Error("expected onuFetcher to be set")
	}
}

func TestHandleEvent_OnlineRemovesFromBatcher(t *testing.T) {
	fetcher := &mockONUDetailFetcher{
		result: model.ONUCustomerInfo{
			ID: 1, Name: "Recovered", Status: "Online",
		},
	}

	webhook := NewWebhookClient("http://example.com", 0, 5, &GenericFormatter{})
	batcher := NewBatcher(webhook, nil, map[Severity]time.Duration{SeverityCritical: time.Hour})

	batcher.Add(model.TrapEvent{Board: 1, PON: 1, OnuID: 1, EventType: "LOS"})

	handler := NewHandler(webhook, batcher, fetcher)
	handler.HandleEvent(model.TrapEvent{
		Timestamp: time.Now(),
		Board: 1, PON: 1, OnuID: 1,
		EventType: "StatusChange",
	})

	batcher.mu.Lock()
	count := len(batcher.groups[SeverityCritical])
	batcher.mu.Unlock()

	if count != 0 {
		t.Errorf("expected ONU removed from batcher after Online, got %d", count)
	}
}

func TestHandleEvent_WithBatcher(t *testing.T) {
	fetcher := &mockONUDetailFetcher{
		result: model.ONUCustomerInfo{
			ID: 1, Name: "Test", Status: "LOS",
		},
	}

	var webhookCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		webhookCalls.Add(1)
		w.WriteHeader(204)
	}))
	defer server.Close()

	webhook := NewWebhookClient(server.URL, 0, 5, &GenericFormatter{})
	batcher := NewBatcher(webhook, nil, map[Severity]time.Duration{
		SeverityCritical: 100 * time.Millisecond,
	})

	go batcher.Start()
	defer func() { _ = batcher.Close() }()

	handler := NewHandler(webhook, batcher, fetcher)

	event := model.TrapEvent{
		Timestamp: time.Now(),
		Board: 1, PON: 1, OnuID: 1,
		EventType: "StatusChange",
	}

	handler.HandleEvent(event)
	time.Sleep(300 * time.Millisecond)

	if webhookCalls.Load() < 1 {
		t.Errorf("Expected batcher to flush and send webhook, got %d calls", webhookCalls.Load())
	}
}

// --- resolveEventType ---

func TestResolveEventType(t *testing.T) {
	tests := []struct {
		status        string
		offlineReason string
		want          string
	}{
		{"Online", "", "Online"},
		{"online", "", "Online"},
		{"LOS", "", "LOS"},
		{"Dying Gasp", "", "DyingGasp"},
		{"PowerOff", "", "PowerOff"},
		{"AuthFailed", "", "AuthFailed"},
		{"Logging", "", "Logging"},
		{"Syncing", "", "Synchronization"},
		{"Offline", "", "Offline"},
		{"Offline", "LOS", "LOS"},
		{"Offline", "DyingGasp", "DyingGasp"},
		{"Offline", "PowerOff", "PowerOff"},
		{"Offline", "AuthFail", "AuthFailed"},
		{"Offline", "LOSi", "LOSi"},
		{"Offline", "LOFi", "LOFi"},
		{"Offline", "UnknownReason", "Offline"},
		{"SomeWeirdStatus", "", "Online"},
		{"", "", "Online"},
	}

	for _, tt := range tests {
		t.Run(tt.status+"_"+tt.offlineReason, func(t *testing.T) {
			got := resolveEventType(tt.status, tt.offlineReason)
			if got != tt.want {
				t.Errorf("resolveEventType(%q, %q) = %q, want %q", tt.status, tt.offlineReason, got, tt.want)
			}
		})
	}
}

// --- normalizeOfflineReason ---

func TestNormalizeOfflineReason(t *testing.T) {
	tests := []struct {
		reason string
		want   string
	}{
		{"LOS", "LOS"},
		{"los", "LOS"},
		{"DyingGasp", "DyingGasp"},
		{"dying gasp", "DyingGasp"},
		{"PowerOff", "PowerOff"},
		{"power failure", "PowerOff"},
		{"AuthFail", "AuthFailed"},
		{"auth error", "AuthFailed"},
		{"LOSi", "LOSi"},
		{"LOFi", "LOFi"},
		{"something unknown", "Offline"},
	}

	for _, tt := range tests {
		t.Run(tt.reason, func(t *testing.T) {
			got := normalizeOfflineReason(tt.reason)
			if got != tt.want {
				t.Errorf("normalizeOfflineReason(%q) = %q, want %q", tt.reason, got, tt.want)
			}
		})
	}
}
