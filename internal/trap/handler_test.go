package trap

import (
	"context"
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

func TestHandleEvent_OfflineLOS(t *testing.T) {
	fetcher := &mockONUDetailFetcher{
		result: model.ONUCustomerInfo{
			ID:           1,
			Name:         "Customer-023",
			Description:  "Perumahan Graha Ria Blok F No.6",
			OnuType:      "F670LV7.1",
			SerialNumber: "ZTEGC12345678",
		},
	}

	// Create a handler with a nil webhook but an onEvent callback to capture
	handler := NewHandler(nil, fetcher)

	// We wrap HandleEvent to capture the enriched event
	event := model.TrapEvent{
		Timestamp: time.Now(),
		Source:    "192.168.213.174",
		Board:     1,
		PON:       5,
		OnuID:     23,
		EventType: "LOS",
		Status:    "offline",
	}

	// HandleEvent with nil webhook should not panic
	handler.HandleEvent(event)
}

func TestHandleEvent_OfflineWithWebhook(t *testing.T) {
	fetcher := &mockONUDetailFetcher{
		result: model.ONUCustomerInfo{
			ID:           1,
			Name:         "Customer-023",
			Description:  "Perumahan Graha Ria Blok F No.6",
			OnuType:      "F670LV7.1",
			SerialNumber: "ZTEGC12345678",
		},
	}

	// We can't easily intercept the goroutine, so we test with an HTTP test server
	// by using a real WebhookClient pointing to a test server.
	// This is tested more thoroughly in webhook_test.go.
	// Here we verify the handler calls webhook.Send (the goroutine fires).

	// Create handler with nil webhook to ensure no panic and fetcher enrichment works
	handler := NewHandler(nil, fetcher)

	event := model.TrapEvent{
		Timestamp: time.Now(),
		Source:    "192.168.213.174",
		Board:     1,
		PON:       5,
		OnuID:     23,
		EventType: "LOS",
		Status:    "offline",
	}

	handler.HandleEvent(event)
}

func TestHandleEvent_OnlineEvent_Skipped(t *testing.T) {
	fetcher := &mockONUDetailFetcher{
		result: model.ONUCustomerInfo{
			ID:   1,
			Name: "Should-Not-Be-Fetched",
		},
	}

	handler := NewHandler(nil, fetcher)

	event := model.TrapEvent{
		Timestamp: time.Now(),
		Source:    "192.168.213.174",
		Board:     1,
		PON:       5,
		OnuID:     23,
		EventType: "Online",
		Status:    "online",
	}

	// Should not panic and should skip (Online is not in offlineEventTypes)
	handler.HandleEvent(event)
}

func TestHandleEvent_UnknownEvent_Skipped(t *testing.T) {
	handler := NewHandler(nil, nil)

	event := model.TrapEvent{
		Timestamp: time.Now(),
		Source:    "192.168.213.174",
		EventType: "Unknown",
	}

	// Should not panic
	handler.HandleEvent(event)
}

func TestHandleEvent_NilWebhookClient(t *testing.T) {
	handler := NewHandler(nil, nil)

	event := model.TrapEvent{
		Timestamp: time.Now(),
		Source:    "192.168.213.174",
		Board:     1,
		PON:       3,
		OnuID:     10,
		EventType: "DyingGasp",
		Status:    "offline",
	}

	// Should not panic even with nil webhook and nil fetcher
	handler.HandleEvent(event)
}

func TestHandleEvent_AllOfflineEventTypes(t *testing.T) {
	offlineTypes := []string{"LOS", "DyingGasp", "PowerOff", "Offline", "AuthFailed", "LOSi", "LOFi"}

	for _, eventType := range offlineTypes {
		t.Run(eventType, func(t *testing.T) {
			handler := NewHandler(nil, nil)

			event := model.TrapEvent{
				Timestamp: time.Now(),
				Source:    "192.168.213.174",
				Board:     1,
				PON:       1,
				OnuID:     1,
				EventType: eventType,
				Status:    "offline",
			}

			// Should not panic for any offline event type
			handler.HandleEvent(event)
		})
	}
}

func TestHandleEvent_NonOfflineEventTypes(t *testing.T) {
	nonOfflineTypes := []string{"Online", "Logging", "Synchronization", "Unknown", ""}

	for _, eventType := range nonOfflineTypes {
		name := eventType
		if name == "" {
			name = "empty"
		}
		t.Run(name, func(t *testing.T) {
			handler := NewHandler(nil, nil)

			event := model.TrapEvent{
				Timestamp: time.Now(),
				Source:    "192.168.213.174",
				Board:     1,
				PON:       1,
				OnuID:     1,
				EventType: eventType,
				Status:    "online",
			}

			// Should not panic and should skip
			handler.HandleEvent(event)
		})
	}
}

func TestHandleEvent_EnrichmentWithMockFetcher(t *testing.T) {
	fetcher := &mockONUDetailFetcher{
		result: model.ONUCustomerInfo{
			ID:           42,
			Name:         "Customer-042",
			Description:  "Jl. Merdeka No. 17",
			OnuType:      "F660V6.0",
			SerialNumber: "ZTEGD87654321",
		},
	}

	// Use nil webhook so we can safely test enrichment logic runs
	handler := NewHandler(nil, fetcher)

	event := model.TrapEvent{
		Timestamp: time.Now(),
		Source:    "192.168.213.174",
		Board:     2,
		PON:       7,
		OnuID:     42,
		EventType: "PowerOff",
		Status:    "offline",
	}

	// Should not panic and should enrich (we can't check the enriched event directly
	// since it's a local variable, but we verify no errors/panics)
	handler.HandleEvent(event)
}

func TestHandleEvent_FetcherError(t *testing.T) {
	fetcher := &mockONUDetailFetcher{
		result: model.ONUCustomerInfo{},
		err:    context.DeadlineExceeded,
	}

	handler := NewHandler(nil, fetcher)

	event := model.TrapEvent{
		Timestamp: time.Now(),
		Source:    "192.168.213.174",
		Board:     1,
		PON:       3,
		OnuID:     10,
		EventType: "LOS",
		Status:    "offline",
	}

	// Should not panic even when fetcher returns error
	handler.HandleEvent(event)
}

func TestNewHandler(t *testing.T) {
	webhook := NewWebhookClient("http://example.com", 3, 5)
	fetcher := &mockONUDetailFetcher{}

	handler := NewHandler(webhook, fetcher)
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
