package trap

import (
	"testing"
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
