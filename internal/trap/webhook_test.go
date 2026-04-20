package trap

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/model"
)

func TestNewWebhookClient(t *testing.T) {
	client := NewWebhookClient("http://example.com/webhook", 3, 10, nil)

	if client == nil {
		t.Fatal("expected non-nil WebhookClient")
	}
	if client.url != "http://example.com/webhook" {
		t.Errorf("expected url http://example.com/webhook, got %s", client.url)
	}
	if client.maxRetries != 3 {
		t.Errorf("expected maxRetries 3, got %d", client.maxRetries)
	}
	if client.timeout != 10*time.Second {
		t.Errorf("expected timeout 10s, got %v", client.timeout)
	}
	if client.client == nil {
		t.Fatal("expected http.Client to be initialized")
	}
}

func TestWebhookSend_Success(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewWebhookClient(server.URL, 3, 5, nil)

	event := model.TrapEvent{
		Timestamp: time.Now(),
		Source:    "192.168.213.174",
		Board:     1,
		PON:       5,
		OnuID:     23,
		EventType: "LOS",
		Status:    "offline",
		Name:      "Customer-023",
	}

	client.Send(event)

	if count := atomic.LoadInt32(&requestCount); count != 1 {
		t.Errorf("expected 1 request, got %d", count)
	}
}

func TestWebhookSend_RetryOnFailure(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)

		if count <= 2 {
			// First two attempts fail
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// Third attempt succeeds
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewWebhookClient(server.URL, 3, 5, nil)

	event := model.TrapEvent{
		Timestamp: time.Now(),
		Source:    "192.168.213.174",
		Board:     1,
		PON:       5,
		OnuID:     23,
		EventType: "LOS",
		Status:    "offline",
	}

	client.Send(event)

	if count := atomic.LoadInt32(&requestCount); count != 3 {
		t.Errorf("expected 3 requests (2 failures + 1 success), got %d", count)
	}
}

func TestWebhookSend_AllRetriesExhausted(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewWebhookClient(server.URL, 2, 5, nil)

	event := model.TrapEvent{
		Timestamp: time.Now(),
		Source:    "192.168.213.174",
		Board:     1,
		PON:       5,
		OnuID:     23,
		EventType: "DyingGasp",
		Status:    "offline",
	}

	client.Send(event)

	// maxRetries=2 means 1 initial + 2 retries = 3 total requests
	if count := atomic.LoadInt32(&requestCount); count != 3 {
		t.Errorf("expected 3 requests (initial + 2 retries), got %d", count)
	}
}

func TestWebhookSend_ConnectionError(t *testing.T) {
	// Use a server that we immediately close to simulate connection errors
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	serverURL := server.URL
	server.Close() // Close immediately to cause connection errors

	client := NewWebhookClient(serverURL, 1, 1, nil)

	event := model.TrapEvent{
		Timestamp: time.Now(),
		Source:    "192.168.213.174",
		Board:     1,
		PON:       1,
		OnuID:     1,
		EventType: "LOS",
		Status:    "offline",
	}

	// Should not panic even when server is unreachable
	client.Send(event)
}

func TestWebhookSend_Status201Accepted(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusCreated) // 201 is still 2xx success
	}))
	defer server.Close()

	client := NewWebhookClient(server.URL, 3, 5, nil)

	event := model.TrapEvent{
		Timestamp: time.Now(),
		Source:    "192.168.213.174",
		Board:     1,
		PON:       1,
		OnuID:     1,
		EventType: "PowerOff",
		Status:    "offline",
	}

	client.Send(event)

	if count := atomic.LoadInt32(&requestCount); count != 1 {
		t.Errorf("expected 1 request (201 is success), got %d", count)
	}
}
