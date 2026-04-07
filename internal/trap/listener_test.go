package trap

import (
	"net"
	"testing"
	"time"

	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/model"
	"github.com/gosnmp/gosnmp"
)

func TestNewListener(t *testing.T) {
	cfg := ListenerConfig{
		Port:      1620,
		Community: "public",
	}

	l := NewListener(cfg)

	if l == nil {
		t.Fatal("expected non-nil Listener")
	}
	if l.addr != "0.0.0.0:1620" {
		t.Errorf("expected addr 0.0.0.0:1620, got %s", l.addr)
	}
	if l.config.Port != 1620 {
		t.Errorf("expected port 1620, got %d", l.config.Port)
	}
	if l.config.Community != "public" {
		t.Errorf("expected community public, got %s", l.config.Community)
	}
	if l.tl == nil {
		t.Fatal("expected trap listener to be initialized")
	}
	if l.tl.Params == nil {
		t.Fatal("expected Params to be set")
	}
	if l.tl.Params.Community != "public" {
		t.Errorf("expected Params.Community public, got %s", l.tl.Params.Community)
	}
}

func TestNewListener_CustomPort(t *testing.T) {
	cfg := ListenerConfig{
		Port:      9999,
		Community: "private",
	}

	l := NewListener(cfg)
	if l.addr != "0.0.0.0:9999" {
		t.Errorf("expected addr 0.0.0.0:9999, got %s", l.addr)
	}
}

func TestParseOnuIndex(t *testing.T) {
	tests := []struct {
		name      string
		fullOID   string
		prefix    string
		wantBoard int
		wantPON   int
		wantOnuID int
	}{
		{
			name:      "board1_pon1",
			fullOID:   OIDOnuStatus + ".285278465.23",
			prefix:    OIDOnuStatus,
			wantBoard: 1,
			wantPON:   1,
			wantOnuID: 23,
		},
		{
			name:      "board1_pon6",
			fullOID:   OIDOnuStatus + ".285278470.5",
			prefix:    OIDOnuStatus,
			wantBoard: 1,
			wantPON:   6,
			wantOnuID: 5,
		},
		{
			name:      "board2_pon1",
			fullOID:   OIDOnuStatus + ".285278721.10",
			prefix:    OIDOnuStatus,
			wantBoard: 2,
			wantPON:   1,
			wantOnuID: 10,
		},
		{
			name:      "board2_pon8",
			fullOID:   OIDOnuStatus + ".285278728.1",
			prefix:    OIDOnuStatus,
			wantBoard: 2,
			wantPON:   8,
			wantOnuID: 1,
		},
		{
			name:      "board1_no_onu_id",
			fullOID:   OIDOnuStatus + ".285278465",
			prefix:    OIDOnuStatus,
			wantBoard: 1,
			wantPON:   1,
			wantOnuID: 0,
		},
		{
			name:      "invalid_encoded_value",
			fullOID:   OIDOnuStatus + ".999999999.5",
			prefix:    OIDOnuStatus,
			wantBoard: 0,
			wantPON:   0,
			wantOnuID: 5,
		},
		{
			name:      "non_numeric_suffix",
			fullOID:   OIDOnuStatus + ".abc",
			prefix:    OIDOnuStatus,
			wantBoard: 0,
			wantPON:   0,
			wantOnuID: 0,
		},
		{
			name:      "empty_suffix",
			fullOID:   OIDOnuStatus,
			prefix:    OIDOnuStatus,
			wantBoard: 0,
			wantPON:   0,
			wantOnuID: 0,
		},
		{
			name:      "with_onu_index_prefix",
			fullOID:   OIDOnuIndex + ".285278465.42",
			prefix:    OIDOnuIndex,
			wantBoard: 1,
			wantPON:   1,
			wantOnuID: 42,
		},
		{
			name:      "default_branch_non_numeric_last_part",
			fullOID:   OIDOnuStatus + ".999999999.abc",
			prefix:    OIDOnuStatus,
			wantBoard: 0,
			wantPON:   0,
			wantOnuID: 0,
		},
		{
			name:      "board1_pon16",
			fullOID:   OIDOnuStatus + ".285278480.1",
			prefix:    OIDOnuStatus,
			wantBoard: 1,
			wantPON:   16,
			wantOnuID: 1,
		},
		{
			name:      "board2_pon16",
			fullOID:   OIDOnuStatus + ".285278736.1",
			prefix:    OIDOnuStatus,
			wantBoard: 2,
			wantPON:   16,
			wantOnuID: 1,
		},
		{
			name:      "board1_pon1_non_numeric_onuid",
			fullOID:   OIDOnuStatus + ".285278465.xyz",
			prefix:    OIDOnuStatus,
			wantBoard: 1,
			wantPON:   1,
			wantOnuID: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			board, pon, onuID := parseOnuIndex(tt.fullOID, tt.prefix)
			if board != tt.wantBoard {
				t.Errorf("board: got %d, want %d", board, tt.wantBoard)
			}
			if pon != tt.wantPON {
				t.Errorf("pon: got %d, want %d", pon, tt.wantPON)
			}
			if onuID != tt.wantOnuID {
				t.Errorf("onuID: got %d, want %d", onuID, tt.wantOnuID)
			}
		})
	}
}

func TestMapStatus(t *testing.T) {
	tests := []struct {
		code       int
		wantStatus string
		wantEvent  string
	}{
		{1, "logging", "Logging"},
		{2, "offline", "LOS"},
		{3, "syncing", "Synchronization"},
		{4, "online", "Online"},
		{5, "offline", "DyingGasp"},
		{6, "offline", "AuthFailed"},
		{7, "offline", "Offline"},
		{0, "unknown", "Unknown"},
		{99, "unknown", "Unknown"},
		{-1, "unknown", "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.wantEvent, func(t *testing.T) {
			status, event := mapStatus(tt.code)
			if status != tt.wantStatus {
				t.Errorf("status: got %q, want %q", status, tt.wantStatus)
			}
			if event != tt.wantEvent {
				t.Errorf("event: got %q, want %q", event, tt.wantEvent)
			}
		})
	}
}

func TestMapOfflineReason(t *testing.T) {
	tests := []struct {
		code int
		want string
	}{
		{1, "Unknown"},
		{2, "LOS"},
		{3, "LOSi"},
		{4, "LOFi"},
		{5, "SFi"},
		{6, "LOAi"},
		{7, "LOAMi"},
		{8, "AuthFail"},
		{9, "PowerOff"},
		{10, "DeactivateSuccess"},
		{11, "DeactivateFail"},
		{12, "Reboot"},
		{13, "Shutdown"},
		{0, "Unknown"},
		{99, "Unknown"},
		{-1, "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := mapOfflineReason(tt.code)
			if got != tt.want {
				t.Errorf("mapOfflineReason(%d) = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

func TestClose_WithoutStart(t *testing.T) {
	cfg := ListenerConfig{
		Port:      1620,
		Community: "public",
	}

	l := NewListener(cfg)

	// Close without Start should not panic
	err := l.Close()
	if err != nil {
		t.Errorf("expected no error on close, got %v", err)
	}
}

func TestHandleTrap_ONUStatus(t *testing.T) {
	var received model.TrapEvent
	listener := NewListener(ListenerConfig{
		Port:      1620,
		Community: "public",
		OnEvent: func(event model.TrapEvent) {
			received = event
		},
	})

	addr := &net.UDPAddr{IP: net.ParseIP("192.168.1.1"), Port: 162}
	packet := &gosnmp.SnmpPacket{
		Variables: []gosnmp.SnmpPDU{
			{
				Name:  ".1.3.6.1.4.1.3902.1082.500.10.2.3.8.1.4.285278465.23",
				Type:  gosnmp.Integer,
				Value: 2, // LOS
			},
		},
	}

	listener.handleTrap(packet, addr)

	if received.Board != 1 || received.PON != 1 || received.OnuID != 23 {
		t.Errorf("Expected board=1 pon=1 onu=23, got board=%d pon=%d onu=%d", received.Board, received.PON, received.OnuID)
	}
	if received.EventType != "LOS" {
		t.Errorf("Expected LOS, got %s", received.EventType)
	}
	if received.Source != "192.168.1.1" {
		t.Errorf("Expected source 192.168.1.1, got %s", received.Source)
	}
}

func TestHandleTrap_ONUOfflineReason(t *testing.T) {
	var received model.TrapEvent
	listener := NewListener(ListenerConfig{
		Port:      1620,
		Community: "public",
		OnEvent: func(event model.TrapEvent) {
			received = event
		},
	})

	addr := &net.UDPAddr{IP: net.ParseIP("10.0.0.1"), Port: 162}
	packet := &gosnmp.SnmpPacket{
		Variables: []gosnmp.SnmpPDU{
			{
				Name:  ".1.3.6.1.4.1.3902.1082.500.10.2.3.8.1.4.285278465.10",
				Type:  gosnmp.Integer,
				Value: 5, // DyingGasp
			},
			{
				Name:  ".1.3.6.1.4.1.3902.1082.500.10.2.3.8.1.7.285278465.10",
				Type:  gosnmp.Integer,
				Value: 9, // PowerOff
			},
		},
	}

	listener.handleTrap(packet, addr)

	if received.EventType != "PowerOff" {
		t.Errorf("Expected PowerOff from offline reason, got %s", received.EventType)
	}
	if received.Board != 1 || received.PON != 1 || received.OnuID != 10 {
		t.Errorf("Expected board=1 pon=1 onu=10, got board=%d pon=%d onu=%d", received.Board, received.PON, received.OnuID)
	}
}

func TestHandleTrap_ONUIndex_ByteName(t *testing.T) {
	var received model.TrapEvent
	listener := NewListener(ListenerConfig{
		Port:      1620,
		Community: "public",
		OnEvent: func(event model.TrapEvent) {
			received = event
		},
	})

	addr := &net.UDPAddr{IP: net.ParseIP("192.168.1.1"), Port: 162}
	packet := &gosnmp.SnmpPacket{
		Variables: []gosnmp.SnmpPDU{
			{
				Name:  ".1.3.6.1.4.1.3902.1082.500.10.2.3.3.1.2.285278721.5",
				Type:  gosnmp.OctetString,
				Value: []byte("ONU-Customer-1"),
			},
		},
	}

	listener.handleTrap(packet, addr)

	if received.Board != 2 || received.PON != 1 || received.OnuID != 5 {
		t.Errorf("Expected board=2 pon=1 onu=5, got board=%d pon=%d onu=%d", received.Board, received.PON, received.OnuID)
	}
	if received.Description != "ONU-Customer-1" {
		t.Errorf("Expected description ONU-Customer-1, got %s", received.Description)
	}
}

func TestHandleTrap_ONUIndex_StringName(t *testing.T) {
	var received model.TrapEvent
	listener := NewListener(ListenerConfig{
		Port:      1620,
		Community: "public",
		OnEvent: func(event model.TrapEvent) {
			received = event
		},
	})

	addr := &net.UDPAddr{IP: net.ParseIP("192.168.1.1"), Port: 162}
	packet := &gosnmp.SnmpPacket{
		Variables: []gosnmp.SnmpPDU{
			{
				Name:  ".1.3.6.1.4.1.3902.1082.500.10.2.3.3.1.2.285278470.3",
				Type:  gosnmp.OctetString,
				Value: "ONU-String-Name",
			},
		},
	}

	listener.handleTrap(packet, addr)

	if received.Board != 1 || received.PON != 6 || received.OnuID != 3 {
		t.Errorf("Expected board=1 pon=6 onu=3, got board=%d pon=%d onu=%d", received.Board, received.PON, received.OnuID)
	}
	if received.Description != "ONU-String-Name" {
		t.Errorf("Expected description ONU-String-Name, got %s", received.Description)
	}
}

func TestHandleTrap_UnknownOIDs(t *testing.T) {
	var received model.TrapEvent
	listener := NewListener(ListenerConfig{
		Port:      1620,
		Community: "public",
		OnEvent: func(event model.TrapEvent) {
			received = event
		},
	})

	addr := &net.UDPAddr{IP: net.ParseIP("10.0.0.5"), Port: 162}
	packet := &gosnmp.SnmpPacket{
		Variables: []gosnmp.SnmpPDU{
			{
				Name:  ".1.3.6.1.2.1.1.3.0",
				Type:  gosnmp.TimeTicks,
				Value: 12345,
			},
		},
	}

	listener.handleTrap(packet, addr)

	if received.EventType != "unknown" {
		t.Errorf("Expected event_type unknown, got %s", received.EventType)
	}
	if received.Source != "10.0.0.5" {
		t.Errorf("Expected source 10.0.0.5, got %s", received.Source)
	}
}

func TestHandleTrap_EmptyVariables(t *testing.T) {
	var received model.TrapEvent
	listener := NewListener(ListenerConfig{
		Port:      1620,
		Community: "public",
		OnEvent: func(event model.TrapEvent) {
			received = event
		},
	})

	addr := &net.UDPAddr{IP: net.ParseIP("192.168.1.1"), Port: 162}
	packet := &gosnmp.SnmpPacket{
		Variables: []gosnmp.SnmpPDU{},
	}

	listener.handleTrap(packet, addr)

	if received.EventType != "unknown" {
		t.Errorf("Expected event_type unknown for empty vars, got %s", received.EventType)
	}
}

func TestHandleTrap_NilOnEvent(t *testing.T) {
	listener := NewListener(ListenerConfig{
		Port:      1620,
		Community: "public",
		OnEvent:   nil,
	})

	addr := &net.UDPAddr{IP: net.ParseIP("192.168.1.1"), Port: 162}
	packet := &gosnmp.SnmpPacket{
		Variables: []gosnmp.SnmpPDU{
			{
				Name:  ".1.3.6.1.4.1.3902.1082.500.10.2.3.8.1.4.285278465.1",
				Type:  gosnmp.Integer,
				Value: 4,
			},
		},
	}

	// Should not panic even with nil OnEvent
	listener.handleTrap(packet, addr)
}

func TestHandleTrap_StatusWithDescription(t *testing.T) {
	// Test the auto-generated description branch (board > 0 but no description set)
	var received model.TrapEvent
	listener := NewListener(ListenerConfig{
		Port:      1620,
		Community: "public",
		OnEvent: func(event model.TrapEvent) {
			received = event
		},
	})

	addr := &net.UDPAddr{IP: net.ParseIP("192.168.1.1"), Port: 162}
	packet := &gosnmp.SnmpPacket{
		Variables: []gosnmp.SnmpPDU{
			{
				Name:  ".1.3.6.1.4.1.3902.1082.500.10.2.3.8.1.4.285278465.1",
				Type:  gosnmp.Integer,
				Value: 4, // Online
			},
		},
	}

	listener.handleTrap(packet, addr)

	if received.Description != "ONU 1/1/1 Online detected" {
		t.Errorf("Expected auto-generated description, got %s", received.Description)
	}
}

func TestListener_StartAndClose(t *testing.T) {
	listener := NewListener(ListenerConfig{
		Port:      0,
		Community: "public",
		OnEvent:   func(event model.TrapEvent) {},
	})

	errCh := make(chan error, 1)
	go func() {
		errCh <- listener.Start()
	}()

	// Wait for listener to be ready
	select {
	case <-listener.Listening():
		// OK - listener is ready
	case <-time.After(5 * time.Second):
		t.Fatal("Listener did not start within 5 seconds")
	}

	// Close should stop it
	listener.Close()
}
