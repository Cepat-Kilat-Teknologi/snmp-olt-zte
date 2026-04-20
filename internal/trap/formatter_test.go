package trap

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/model"
)

var testEvent = model.TrapEvent{
	Timestamp:    time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC),
	Source:       "192.168.213.174",
	Board:        1,
	PON:          5,
	OnuID:        23,
	EventType:    "LOS",
	Status:       "offline",
	Name:         "Customer-023",
	Description:  "Perumahan Graha Ria Blok F No.6",
	OnuType:      "F670LV7.1",
	SerialNumber: "ZTEGC12345678",
	RXPower:      "-22.50",
}

var minimalEvent = model.TrapEvent{
	Timestamp: time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC),
	Source:    "192.168.213.174",
	Board:     1,
	PON:       1,
	OnuID:     1,
	EventType: "LOS",
	Status:    "offline",
}

// --- eventSeverity ---

func TestEventSeverity(t *testing.T) {
	tests := []struct {
		eventType string
		expected  Severity
	}{
		{"LOS", SeverityCritical},
		{"LOSi", SeverityCritical},
		{"LOFi", SeverityCritical},
		{"Offline", SeverityCritical},
		{"AuthFailed", SeverityCritical},
		{"PowerOff", SeverityCritical},
		{"Logging", SeverityHigh},
		{"Synchronization", SeverityHigh},
		{"HighRxPower", SeverityMedium},
		{"LowRxPower", SeverityMedium},
		{"DyingGasp", SeverityLow},
		{"Online", SeverityUnknown},
		{"Unknown", SeverityUnknown},
		{"", SeverityUnknown},
		{"SomethingElse", SeverityUnknown},
	}

	for _, tt := range tests {
		name := tt.eventType
		if name == "" {
			name = "empty"
		}
		t.Run(name, func(t *testing.T) {
			if got := eventSeverity(tt.eventType); got != tt.expected {
				t.Errorf("eventSeverity(%q) = %d, want %d", tt.eventType, got, tt.expected)
			}
		})
	}
}

// --- severityEmoji ---

func TestSeverityEmoji(t *testing.T) {
	tests := []struct {
		sev      Severity
		expected string
	}{
		{SeverityCritical, "\U0001F534"},
		{SeverityHigh, "\U0001F7E0"},
		{SeverityMedium, "\U0001F7E1"},
		{SeverityLow, "\U0001F535"},
		{SeverityUnknown, "\u26AA"},
		{Severity(99), "\u26AA"},
	}

	for _, tt := range tests {
		if got := severityEmoji(tt.sev); got != tt.expected {
			t.Errorf("severityEmoji(%d) = %q, want %q", tt.sev, got, tt.expected)
		}
	}
}

// --- severityLabel ---

func TestSeverityLabel(t *testing.T) {
	tests := []struct {
		sev      Severity
		expected string
	}{
		{SeverityCritical, "CRITICAL"},
		{SeverityHigh, "HIGH"},
		{SeverityMedium, "MEDIUM"},
		{SeverityLow, "LOW"},
		{SeverityUnknown, "UNKNOWN"},
		{Severity(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		if got := severityLabel(tt.sev); got != tt.expected {
			t.Errorf("severityLabel(%d) = %q, want %q", tt.sev, got, tt.expected)
		}
	}
}

// --- severityAction ---

func TestSeverityAction(t *testing.T) {
	tests := []struct {
		sev  Severity
		want string
	}{
		{SeverityCritical, "Wajib visit ke Customer maksimal 1x24 jam"},
		{SeverityHigh, "Wajib visit maksimal 1x24 jam jika Hard Restart tidak Solved"},
		{SeverityMedium, "Wajib visit maksimal 2x24 jam setelah notifikasi"},
		{SeverityLow, "Koordinasi kepada Customer untuk memastikan tidak ada kendala kelistrikan"},
		{SeverityUnknown, ""},
		{Severity(99), ""},
	}

	for _, tt := range tests {
		if got := severityAction(tt.sev); got != tt.want {
			t.Errorf("severityAction(%d) = %q, want %q", tt.sev, got, tt.want)
		}
	}
}

// --- severityColorDiscord ---

func TestSeverityColorDiscord(t *testing.T) {
	tests := []struct {
		sev      Severity
		expected int
	}{
		{SeverityCritical, 0xFF0000},
		{SeverityHigh, 0xFF8C00},
		{SeverityMedium, 0xFFD700},
		{SeverityLow, 0x3498DB},
		{SeverityUnknown, 0x95A5A6},
		{Severity(99), 0x95A5A6},
	}

	for _, tt := range tests {
		if got := severityColorDiscord(tt.sev); got != tt.expected {
			t.Errorf("severityColorDiscord(%d) = %d, want %d", tt.sev, got, tt.expected)
		}
	}
}

// --- severityColorHex ---

func TestSeverityColorHex(t *testing.T) {
	tests := []struct {
		sev      Severity
		expected string
	}{
		{SeverityCritical, "#FF0000"},
		{SeverityHigh, "#FF8C00"},
		{SeverityMedium, "#FFD700"},
		{SeverityLow, "#3498DB"},
		{SeverityUnknown, "#95A5A6"},
		{Severity(99), "#95A5A6"},
	}

	for _, tt := range tests {
		if got := severityColorHex(tt.sev); got != tt.expected {
			t.Errorf("severityColorHex(%d) = %q, want %q", tt.sev, got, tt.expected)
		}
	}
}

// --- eventTitle ---

func TestEventTitle(t *testing.T) {
	tests := []struct {
		name     string
		event    model.TrapEvent
		contains []string
	}{
		{
			name:     "LOS_critical",
			event:    model.TrapEvent{EventType: "LOS"},
			contains: []string{"CRITICAL", "LOS"},
		},
		{
			name:     "Logging_high_stuck",
			event:    model.TrapEvent{EventType: "Logging"},
			contains: []string{"HIGH", "STUCK"},
		},
		{
			name:     "Synchronization_high_stuck",
			event:    model.TrapEvent{EventType: "Synchronization"},
			contains: []string{"HIGH", "STUCK"},
		},
		{
			name:     "HighRxPower_medium",
			event:    model.TrapEvent{EventType: "HighRxPower"},
			contains: []string{"MEDIUM", "HighRxPower"},
		},
		{
			name:     "DyingGasp_low",
			event:    model.TrapEvent{EventType: "DyingGasp"},
			contains: []string{"LOW", "DyingGasp"},
		},
		{
			name:     "empty_event_type",
			event:    model.TrapEvent{EventType: ""},
			contains: []string{"UNKNOWN", "Event"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title := eventTitle(tt.event)
			for _, substr := range tt.contains {
				if !strings.Contains(title, substr) {
					t.Errorf("eventTitle() = %q, want to contain %q", title, substr)
				}
			}
		})
	}
}

// --- formatTimestampWIB ---

func TestFormatTimestampWIB(t *testing.T) {
	ts := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC) // 10:00 UTC = 17:00 WIB
	result := formatTimestampWIB(ts)

	if !strings.Contains(result, "20-04-2026") {
		t.Errorf("expected date 20-04-2026, got %q", result)
	}
	if !strings.Contains(result, "17:00:00") {
		t.Errorf("expected time 17:00:00, got %q", result)
	}
	if !strings.HasSuffix(result, "WIB") {
		t.Errorf("expected WIB suffix, got %q", result)
	}
}

func TestFormatTimestampWIB_ZeroTime(t *testing.T) {
	result := formatTimestampWIB(time.Time{})
	if !strings.HasSuffix(result, "WIB") {
		t.Errorf("expected WIB suffix for zero time, got %q", result)
	}
}

func TestFormatTimestampWIB_FallbackTimezone(t *testing.T) {
	orig := loadLocation
	loadLocation = func(_ string) (*time.Location, error) {
		return nil, fmt.Errorf("timezone not found")
	}
	defer func() { loadLocation = orig }()

	ts := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
	result := formatTimestampWIB(ts)
	if !strings.Contains(result, "17:00:00") {
		t.Errorf("expected 17:00:00 with fallback WIB (UTC+7), got %q", result)
	}
	if !strings.HasSuffix(result, "WIB") {
		t.Errorf("expected WIB suffix, got %q", result)
	}
}

// --- fieldOrDash ---

func TestFieldOrDash(t *testing.T) {
	if got := fieldOrDash("hello"); got != "hello" {
		t.Errorf("fieldOrDash(\"hello\") = %q, want \"hello\"", got)
	}
	if got := fieldOrDash(""); got != "-" {
		t.Errorf("fieldOrDash(\"\") = %q, want \"-\"", got)
	}
}

// --- DetectPlatform ---

func TestDetectPlatform(t *testing.T) {
	tests := []struct {
		url      string
		expected string
	}{
		{"https://discord.com/api/webhooks/123/abc", "discord"},
		{"https://hooks.slack.com/services/T00/B00/xxx", "slack"},
		{"https://api.telegram.org/bot123456:ABC/sendMessage", "telegram"},
		{"https://api.telegram.org/bot123456:ABC", "telegram"},
		{"https://example.com/webhook", "generic"},
		{"http://localhost:9999/test", "generic"},
		{"", "generic"},
	}

	for _, tt := range tests {
		t.Run(tt.expected+"_"+tt.url, func(t *testing.T) {
			if got := DetectPlatform(tt.url); got != tt.expected {
				t.Errorf("DetectPlatform(%q) = %q, want %q", tt.url, got, tt.expected)
			}
		})
	}
}

// --- buildTelegramURL ---

func TestBuildTelegramURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"https://api.telegram.org/bot123", "https://api.telegram.org/bot123/sendMessage"},
		{"https://api.telegram.org/bot123/", "https://api.telegram.org/bot123/sendMessage"},
		{"https://api.telegram.org/bot123/sendMessage", "https://api.telegram.org/bot123/sendMessage"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := buildTelegramURL(tt.input); got != tt.expected {
				t.Errorf("buildTelegramURL(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// --- NewFormatter factory ---

func TestNewFormatter(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		webhookType string
		chatID      string
		wantType    string
		wantURL     string
	}{
		{
			name: "explicit_discord", url: "https://example.com/hook",
			webhookType: "discord", wantType: "*trap.DiscordFormatter", wantURL: "https://example.com/hook",
		},
		{
			name: "explicit_slack", url: "https://example.com/hook",
			webhookType: "slack", wantType: "*trap.SlackFormatter", wantURL: "https://example.com/hook",
		},
		{
			name: "explicit_telegram_with_chatid", url: "https://api.telegram.org/bot123",
			webhookType: "telegram", chatID: "-100123", wantType: "*trap.TelegramFormatter",
			wantURL: "https://api.telegram.org/bot123/sendMessage",
		},
		{
			name: "explicit_telegram_no_chatid_fallback", url: "https://api.telegram.org/bot123",
			webhookType: "telegram", chatID: "", wantType: "*trap.GenericFormatter",
			wantURL: "https://api.telegram.org/bot123",
		},
		{
			name: "explicit_generic", url: "https://example.com/hook",
			webhookType: "generic", wantType: "*trap.GenericFormatter", wantURL: "https://example.com/hook",
		},
		{
			name: "auto_detect_discord", url: "https://discord.com/api/webhooks/123/abc",
			wantType: "*trap.DiscordFormatter", wantURL: "https://discord.com/api/webhooks/123/abc",
		},
		{
			name: "auto_detect_slack", url: "https://hooks.slack.com/services/T00/B00/xxx",
			wantType: "*trap.SlackFormatter", wantURL: "https://hooks.slack.com/services/T00/B00/xxx",
		},
		{
			name: "auto_detect_telegram_with_chatid", url: "https://api.telegram.org/bot123:ABC",
			chatID: "-100123", wantType: "*trap.TelegramFormatter",
			wantURL: "https://api.telegram.org/bot123:ABC/sendMessage",
		},
		{
			name: "auto_detect_telegram_no_chatid", url: "https://api.telegram.org/bot123:ABC",
			chatID: "", wantType: "*trap.GenericFormatter", wantURL: "https://api.telegram.org/bot123:ABC",
		},
		{
			name: "auto_detect_unknown", url: "https://example.com/alerts",
			wantType: "*trap.GenericFormatter", wantURL: "https://example.com/alerts",
		},
		{
			name: "unknown_type_fallback", url: "https://example.com/hook",
			webhookType: "unknown_platform", wantType: "*trap.GenericFormatter", wantURL: "https://example.com/hook",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter, gotURL := NewFormatter(tt.url, tt.webhookType, tt.chatID)
			gotType := typeString(formatter)
			if gotType != tt.wantType {
				t.Errorf("NewFormatter() type = %s, want %s", gotType, tt.wantType)
			}
			if gotURL != tt.wantURL {
				t.Errorf("NewFormatter() url = %q, want %q", gotURL, tt.wantURL)
			}
		})
	}
}

func typeString(f WebhookFormatter) string {
	switch f.(type) {
	case *DiscordFormatter:
		return "*trap.DiscordFormatter"
	case *SlackFormatter:
		return "*trap.SlackFormatter"
	case *TelegramFormatter:
		return "*trap.TelegramFormatter"
	case *GenericFormatter:
		return "*trap.GenericFormatter"
	default:
		return "unknown"
	}
}

// --- GenericFormatter ---

func TestGenericFormatter_Format(t *testing.T) {
	f := &GenericFormatter{}
	data, err := f.Format(testEvent)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}
	var result model.TrapEvent
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result.EventType != "LOS" {
		t.Errorf("EventType = %q, want LOS", result.EventType)
	}
	if result.Name != "Customer-023" {
		t.Errorf("Name = %q, want Customer-023", result.Name)
	}
}

func TestGenericFormatter_ContentType(t *testing.T) {
	f := &GenericFormatter{}
	if ct := f.ContentType(); ct != "application/json" {
		t.Errorf("ContentType() = %q, want application/json", ct)
	}
}

// --- DiscordFormatter ---

func TestDiscordFormatter_Format_AllFields(t *testing.T) {
	f := &DiscordFormatter{}
	data, err := f.Format(testEvent)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	var payload discordPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(payload.Embeds) != 1 {
		t.Fatalf("expected 1 embed, got %d", len(payload.Embeds))
	}

	embed := payload.Embeds[0]

	if !strings.Contains(embed.Title, "CRITICAL") || !strings.Contains(embed.Title, "LOS") {
		t.Errorf("title = %q, want CRITICAL + LOS", embed.Title)
	}
	if embed.Color != 0xFF0000 {
		t.Errorf("color = %d, want %d (red)", embed.Color, 0xFF0000)
	}
	if !strings.Contains(embed.Footer.Text, "WIB") {
		t.Errorf("footer = %q, want WIB timestamp", embed.Footer.Text)
	}

	fieldMap := make(map[string]string)
	for _, f := range embed.Fields {
		fieldMap[f.Name] = f.Value
	}

	if v := fieldMap["Nama"]; v != "Customer-023" {
		t.Errorf("Nama = %q, want Customer-023", v)
	}
	if v := fieldMap["Event"]; v != "LOS" {
		t.Errorf("Event = %q, want LOS", v)
	}
	if v := fieldMap["Board/PON/ONU"]; v != "1/5/23" {
		t.Errorf("Board/PON/ONU = %q, want 1/5/23", v)
	}
	if v := fieldMap["Serial Number"]; v != "ZTEGC12345678" {
		t.Errorf("Serial Number = %q, want ZTEGC12345678", v)
	}
	if v := fieldMap["Alamat"]; v != "Perumahan Graha Ria Blok F No.6" {
		t.Errorf("Alamat = %q", v)
	}
	if v := fieldMap["Last Offline"]; !strings.Contains(v, "WIB") {
		t.Errorf("Last Offline = %q, want WIB suffix", v)
	}
	if v := fieldMap["\u26A0\uFE0F Action"]; !strings.Contains(v, "1x24 jam") {
		t.Errorf("Action = %q, want action text", v)
	}
}

func TestDiscordFormatter_Format_MinimalFields(t *testing.T) {
	f := &DiscordFormatter{}
	data, err := f.Format(minimalEvent)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	var payload discordPayload
	_ = json.Unmarshal(data, &payload)
	embed := payload.Embeds[0]

	fieldMap := make(map[string]string)
	for _, f := range embed.Fields {
		fieldMap[f.Name] = f.Value
	}

	// Nama should be "-" for empty
	if v := fieldMap["Nama"]; v != "-" {
		t.Errorf("Nama = %q, want '-' for empty", v)
	}
	// No optional fields
	if _, ok := fieldMap["Serial Number"]; ok {
		t.Error("minimal event should not have Serial Number")
	}
	if _, ok := fieldMap["Alamat"]; ok {
		t.Error("minimal event should not have Alamat")
	}
}

func TestDiscordFormatter_Format_ZeroTimestamp(t *testing.T) {
	f := &DiscordFormatter{}
	event := model.TrapEvent{EventType: "LOS", Board: 1, PON: 1, OnuID: 1}
	data, err := f.Format(event)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}
	var payload discordPayload
	_ = json.Unmarshal(data, &payload)
	if payload.Embeds[0].Timestamp == "" {
		t.Error("expected non-empty timestamp even with zero time")
	}
}

func TestDiscordFormatter_ContentType(t *testing.T) {
	f := &DiscordFormatter{}
	if ct := f.ContentType(); ct != "application/json" {
		t.Errorf("ContentType() = %q, want application/json", ct)
	}
}

func TestDiscordFormatter_AllSeverityColors(t *testing.T) {
	f := &DiscordFormatter{}
	events := []struct {
		eventType string
		wantColor int
	}{
		{"LOS", 0xFF0000},
		{"Logging", 0xFF8C00},
		{"HighRxPower", 0xFFD700},
		{"DyingGasp", 0x3498DB},
		{"Unknown", 0x95A5A6},
	}

	for _, tt := range events {
		t.Run(tt.eventType, func(t *testing.T) {
			data, _ := f.Format(model.TrapEvent{Timestamp: time.Now(), EventType: tt.eventType, Board: 1, PON: 1, OnuID: 1})
			var payload discordPayload
			_ = json.Unmarshal(data, &payload)
			if payload.Embeds[0].Color != tt.wantColor {
				t.Errorf("color = %d, want %d", payload.Embeds[0].Color, tt.wantColor)
			}
		})
	}
}

func TestDiscordFormatter_UnknownSeverity_NoAction(t *testing.T) {
	f := &DiscordFormatter{}
	data, _ := f.Format(model.TrapEvent{Timestamp: time.Now(), EventType: "Online", Board: 1, PON: 1, OnuID: 1})
	var payload discordPayload
	_ = json.Unmarshal(data, &payload)
	for _, field := range payload.Embeds[0].Fields {
		if strings.Contains(field.Name, "Action") {
			t.Error("unknown severity should not have Action field")
		}
	}
}

// --- SlackFormatter ---

func TestSlackFormatter_Format_AllFields(t *testing.T) {
	f := &SlackFormatter{}
	data, err := f.Format(testEvent)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	var payload slackPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(payload.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(payload.Attachments))
	}

	att := payload.Attachments[0]
	if att.Color != "#FF0000" {
		t.Errorf("color = %q, want #FF0000", att.Color)
	}

	// header + fields + alamat + tindakan + context = 5 blocks
	if len(att.Blocks) != 5 {
		t.Fatalf("expected 5 blocks, got %d", len(att.Blocks))
	}

	if att.Blocks[0].Type != "header" {
		t.Errorf("block[0].type = %q, want header", att.Blocks[0].Type)
	}
	if !strings.Contains(att.Blocks[0].Text.Text, "LOS") {
		t.Errorf("header text = %q, want to contain LOS", att.Blocks[0].Text.Text)
	}
}

func TestSlackFormatter_Format_MinimalFields(t *testing.T) {
	f := &SlackFormatter{}
	data, _ := f.Format(minimalEvent)
	var payload slackPayload
	_ = json.Unmarshal(data, &payload)

	// header + fields + tindakan + context = 4 blocks (no alamat)
	if len(payload.Attachments[0].Blocks) != 4 {
		t.Errorf("expected 4 blocks (no alamat), got %d", len(payload.Attachments[0].Blocks))
	}
}

func TestSlackFormatter_Format_ZeroTimestamp(t *testing.T) {
	f := &SlackFormatter{}
	event := model.TrapEvent{EventType: "LOS", Board: 1, PON: 1, OnuID: 1}
	data, _ := f.Format(event)
	var payload slackPayload
	_ = json.Unmarshal(data, &payload)
	blocks := payload.Attachments[0].Blocks
	contextBlock := blocks[len(blocks)-1]
	if !strings.Contains(contextBlock.Text.Text, "WIB") {
		t.Errorf("context = %q, want WIB timestamp", contextBlock.Text.Text)
	}
}

func TestSlackFormatter_ContentType(t *testing.T) {
	f := &SlackFormatter{}
	if ct := f.ContentType(); ct != "application/json" {
		t.Errorf("ContentType() = %q, want application/json", ct)
	}
}

func TestSlackFormatter_AllSeverityColors(t *testing.T) {
	f := &SlackFormatter{}
	events := []struct {
		eventType string
		wantColor string
	}{
		{"LOS", "#FF0000"},
		{"Logging", "#FF8C00"},
		{"HighRxPower", "#FFD700"},
		{"DyingGasp", "#3498DB"},
		{"Unknown", "#95A5A6"},
	}

	for _, tt := range events {
		t.Run(tt.eventType, func(t *testing.T) {
			data, _ := f.Format(model.TrapEvent{Timestamp: time.Now(), EventType: tt.eventType, Board: 1, PON: 1, OnuID: 1})
			var payload slackPayload
			_ = json.Unmarshal(data, &payload)
			if payload.Attachments[0].Color != tt.wantColor {
				t.Errorf("color = %q, want %q", payload.Attachments[0].Color, tt.wantColor)
			}
		})
	}
}

func TestSlackFormatter_UnknownSeverity_NoAction(t *testing.T) {
	f := &SlackFormatter{}
	data, _ := f.Format(model.TrapEvent{Timestamp: time.Now(), EventType: "Online", Board: 1, PON: 1, OnuID: 1})
	var payload slackPayload
	_ = json.Unmarshal(data, &payload)
	// header + fields + context = 3 blocks (no tindakan, no alamat)
	if len(payload.Attachments[0].Blocks) != 3 {
		t.Errorf("expected 3 blocks (no tindakan, no alamat), got %d", len(payload.Attachments[0].Blocks))
	}
}

// --- TelegramFormatter ---

func TestTelegramFormatter_Format_AllFields(t *testing.T) {
	f := &TelegramFormatter{chatID: "-100123456"}
	data, err := f.Format(testEvent)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	var payload telegramPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if payload.ChatID != "-100123456" {
		t.Errorf("chat_id = %q, want -100123456", payload.ChatID)
	}
	if payload.ParseMode != "HTML" {
		t.Errorf("parse_mode = %q, want HTML", payload.ParseMode)
	}

	text := payload.Text
	for _, expected := range []string{
		"CRITICAL", "LOS", "Customer-023", "ZTEGC12345678", "F670LV7.1",
		"-22.50 dBm", "Perumahan Graha Ria Blok F No.6",
		"1x24 jam", "WIB", "1/5/23",
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("text missing %q", expected)
		}
	}
}

func TestTelegramFormatter_Format_MinimalFields(t *testing.T) {
	f := &TelegramFormatter{chatID: "-100123"}
	data, _ := f.Format(minimalEvent)
	var payload telegramPayload
	_ = json.Unmarshal(data, &payload)

	for _, absent := range []string{"Serial:", "ONU Type:", "RX Power:", "Alamat:"} {
		if strings.Contains(payload.Text, "<b>"+absent+"</b>") {
			t.Errorf("minimal event should not contain %s", absent)
		}
	}
}

func TestTelegramFormatter_Format_ZeroTimestamp(t *testing.T) {
	f := &TelegramFormatter{chatID: "-100123"}
	event := model.TrapEvent{EventType: "LOS", Board: 1, PON: 1, OnuID: 1}
	data, _ := f.Format(event)
	var payload telegramPayload
	_ = json.Unmarshal(data, &payload)
	if !strings.Contains(payload.Text, "WIB") {
		t.Error("expected WIB in timestamp")
	}
}

func TestTelegramFormatter_ContentType(t *testing.T) {
	f := &TelegramFormatter{chatID: "-100123"}
	if ct := f.ContentType(); ct != "application/json" {
		t.Errorf("ContentType() = %q, want application/json", ct)
	}
}

func TestTelegramFormatter_UnknownSeverity_NoAction(t *testing.T) {
	f := &TelegramFormatter{chatID: "-100123"}
	data, _ := f.Format(model.TrapEvent{Timestamp: time.Now(), EventType: "Online", Board: 1, PON: 1, OnuID: 1})
	var payload telegramPayload
	_ = json.Unmarshal(data, &payload)
	if strings.Contains(payload.Text, "Action") {
		t.Error("unknown severity should not have Action")
	}
}

// --- failingFormatter for testing error path ---

type failingFormatter struct{}

func (f *failingFormatter) Format(_ model.TrapEvent) ([]byte, error) {
	return nil, fmt.Errorf("simulated format error")
}

func (f *failingFormatter) FormatBatch(_ Severity, _ []model.TrapEvent) ([]byte, error) {
	return nil, fmt.Errorf("simulated format error")
}

func (f *failingFormatter) ContentType() string {
	return "application/json"
}

func TestWebhookSend_FormatError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("server should not be called when format fails")
		w.WriteHeader(200)
	}))
	defer server.Close()

	client := NewWebhookClient(server.URL, 0, 5, &failingFormatter{})
	client.Send(model.TrapEvent{EventType: "LOS"})
}

// --- WebhookClient nil formatter fallback ---

func TestNewWebhookClient_NilFormatter_FallsBackToGeneric(t *testing.T) {
	client := NewWebhookClient("http://example.com", 0, 5, nil)
	if client.formatter == nil {
		t.Fatal("expected non-nil formatter")
	}
	if _, ok := client.formatter.(*GenericFormatter); !ok {
		t.Errorf("expected GenericFormatter, got %T", client.formatter)
	}
}

func TestNewWebhookClient_WithFormatter(t *testing.T) {
	f := &DiscordFormatter{}
	client := NewWebhookClient("http://example.com", 0, 5, f)
	if client.formatter != f {
		t.Error("expected formatter to be set to provided value")
	}
}

// --- truncate ---

func TestTruncate(t *testing.T) {
	if got := truncate("short", 10); got != "short" {
		t.Errorf("truncate short = %q", got)
	}
	if got := truncate("exactly10!", 10); got != "exactly10!" {
		t.Errorf("truncate exact = %q", got)
	}
	got := truncate("this is too long", 10)
	if !strings.HasSuffix(got, "\u2026") {
		t.Errorf("truncate long = %q, want ellipsis suffix", got)
	}
	if len([]rune(got)) > 10 {
		t.Errorf("truncate long rune len = %d, want <= 10", len([]rune(got)))
	}
}

// --- BuildIntervals ---

func TestBuildIntervals(t *testing.T) {
	m := BuildIntervals(300, 3600, 14400, 28800)
	if len(m) != 4 {
		t.Fatalf("expected 4 intervals, got %d", len(m))
	}
	if m[SeverityCritical] != 300*time.Second {
		t.Errorf("critical = %v", m[SeverityCritical])
	}
	if m[SeverityHigh] != 3600*time.Second {
		t.Errorf("high = %v", m[SeverityHigh])
	}
}

func TestBuildIntervals_ZeroSkipped(t *testing.T) {
	m := BuildIntervals(0, 0, 300, 0)
	if len(m) != 1 {
		t.Fatalf("expected 1 interval, got %d", len(m))
	}
	if _, ok := m[SeverityMedium]; !ok {
		t.Error("expected medium to be set")
	}
}

func TestBuildRepeatIntervals(t *testing.T) {
	m := BuildRepeatIntervals(60, 120, 240, 480)
	if len(m) != 4 {
		t.Fatalf("expected 4, got %d", len(m))
	}
	if m[SeverityCritical] != 60*time.Minute {
		t.Errorf("critical = %v", m[SeverityCritical])
	}
	if m[SeverityLow] != 480*time.Minute {
		t.Errorf("low = %v", m[SeverityLow])
	}
}

func TestBuildRepeatIntervals_ZeroSkipped(t *testing.T) {
	m := BuildRepeatIntervals(0, 0, 60, 0)
	if len(m) != 1 {
		t.Fatalf("expected 1, got %d", len(m))
	}
}

func TestBuildIntervals_AllZero(t *testing.T) {
	m := BuildIntervals(0, 0, 0, 0)
	if len(m) != 0 {
		t.Fatalf("expected 0 intervals, got %d", len(m))
	}
}

// --- batchCategory ---

func TestBatchCategory(t *testing.T) {
	events := []model.TrapEvent{{EventType: "LOS"}}
	if got := batchCategory(SeverityCritical, events); got != "LOS" {
		t.Errorf("critical = %q, want LOS", got)
	}
	if got := batchCategory(SeverityHigh, events); got != "STUCK" {
		t.Errorf("high = %q, want STUCK", got)
	}
	if got := batchCategory(SeverityCritical, nil); got != "Event" {
		t.Errorf("empty = %q, want Event", got)
	}
}

// --- formatLastOnline ---

func TestFormatLastOnline(t *testing.T) {
	e1 := model.TrapEvent{LastOnline: "2026-04-20 17:00:00"}
	if got := formatLastOnline(e1); got != "20-04-2026/17:00:00" {
		t.Errorf("with LastOnline = %q, want 20-04-2026/17:00:00", got)
	}

	e2 := model.TrapEvent{LastOffline: "2026-04-20 18:30:00"}
	if got := formatLastOnline(e2); got != "20-04-2026/18:30:00" {
		t.Errorf("fallback to LastOffline = %q, want 20-04-2026/18:30:00", got)
	}

	e3 := model.TrapEvent{Timestamp: time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)}
	if got := formatLastOnline(e3); !strings.Contains(got, "WIB") {
		t.Errorf("fallback to timestamp = %q, want WIB", got)
	}

	e4 := model.TrapEvent{LastOnline: "invalid-date"}
	if got := formatLastOnline(e4); got != "invalid-date" {
		t.Errorf("invalid date = %q, want raw passthrough", got)
	}
}

// --- FormatBatch tests ---

var batchEvents = []model.TrapEvent{
	{
		Timestamp: time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC),
		Board: 1, PON: 5, OnuID: 23,
		EventType: "LOS", Status: "offline",
		Name: "Budi Santoso", Description: "Perumahan Graha Ria",
		SerialNumber: "ZTEGC12345678", RXPower: "-22.50",
		LastOffline: "2026-04-20 17:00:00",
	},
	{
		Timestamp: time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC),
		Board: 1, PON: 13, OnuID: 40,
		EventType: "LOS", Status: "offline",
		Name: "Widya Sofani", Description: "Kejambon RT 007/003",
		SerialNumber: "ZXICCC94FA71",
		LastOffline: "2026-04-20 17:05:00",
	},
}

func TestDiscordFormatter_FormatBatch(t *testing.T) {
	f := &DiscordFormatter{}
	data, err := f.FormatBatch(SeverityCritical, batchEvents)
	if err != nil {
		t.Fatalf("FormatBatch error = %v", err)
	}

	var payload discordPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	embed := payload.Embeds[0]
	if !strings.Contains(embed.Title, "2 ONU") {
		t.Errorf("title = %q, want 2 ONU", embed.Title)
	}
	if !strings.Contains(embed.Description, "Budi Santoso") {
		t.Errorf("desc missing Budi Santoso")
	}
	if !strings.Contains(embed.Description, "Widya Sofani") {
		t.Errorf("desc missing Widya Sofani")
	}
	if !strings.Contains(embed.Description, "> ") {
		t.Errorf("desc missing quote separator")
	}
	if !strings.Contains(embed.Description, "Action") {
		t.Errorf("desc missing Action")
	}
	if !strings.Contains(embed.Description, "-22.50 dBm") {
		t.Errorf("desc missing RX Power")
	}
}

func TestDiscordFormatter_FormatBatch_NoRxPower(t *testing.T) {
	f := &DiscordFormatter{}
	events := []model.TrapEvent{{Board: 1, PON: 1, OnuID: 1, EventType: "LOS", Name: "Test"}}
	data, _ := f.FormatBatch(SeverityCritical, events)
	if strings.Contains(string(data), "RX Power") {
		t.Error("should not contain RX Power when empty")
	}
}

func TestDiscordFormatter_FormatBatch_UnknownSeverity(t *testing.T) {
	f := &DiscordFormatter{}
	events := []model.TrapEvent{{Board: 1, PON: 1, OnuID: 1, EventType: "Unknown"}}
	data, _ := f.FormatBatch(SeverityUnknown, events)
	if strings.Contains(string(data), "Action") {
		t.Error("unknown severity should not have Action")
	}
}

func TestSlackFormatter_FormatBatch(t *testing.T) {
	f := &SlackFormatter{}
	data, err := f.FormatBatch(SeverityLow, []model.TrapEvent{
		{Board: 1, PON: 2, OnuID: 5, EventType: "DyingGasp", Name: "Test Customer",
			Description: "Jl. Test", RXPower: "-20.00", LastOffline: "2026-04-20 17:00:00"},
	})
	if err != nil {
		t.Fatalf("FormatBatch error = %v", err)
	}

	var payload slackPayload
	_ = json.Unmarshal(data, &payload)

	if payload.Attachments[0].Color != "#3498DB" {
		t.Errorf("color = %q, want blue", payload.Attachments[0].Color)
	}

	blocks := payload.Attachments[0].Blocks
	if !strings.Contains(blocks[0].Text.Text, "1 ONU") {
		t.Errorf("header = %q, want 1 ONU", blocks[0].Text.Text)
	}
	if !strings.Contains(blocks[1].Text.Text, "Test Customer") {
		t.Errorf("body missing customer name")
	}
}

func TestTelegramFormatter_FormatBatch(t *testing.T) {
	f := &TelegramFormatter{chatID: "-100123"}
	data, err := f.FormatBatch(SeverityHigh, []model.TrapEvent{
		{Board: 1, PON: 1, OnuID: 1, EventType: "Logging", Name: "Customer A"},
		{Board: 1, PON: 2, OnuID: 2, EventType: "Logging", Name: "Customer B",
			Description: "Alamat B", RXPower: "-15.00"},
	})
	if err != nil {
		t.Fatalf("FormatBatch error = %v", err)
	}

	var payload telegramPayload
	_ = json.Unmarshal(data, &payload)

	if payload.ChatID != "-100123" {
		t.Errorf("chat_id = %q", payload.ChatID)
	}
	if !strings.Contains(payload.Text, "2 ONU STUCK") {
		t.Errorf("text missing batch title")
	}
	if !strings.Contains(payload.Text, "Customer A") {
		t.Errorf("text missing Customer A")
	}
	if !strings.Contains(payload.Text, "Customer B") {
		t.Errorf("text missing Customer B")
	}
	if !strings.Contains(payload.Text, "===") {
		t.Errorf("text missing separator")
	}
	if !strings.Contains(payload.Text, "Action") {
		t.Errorf("text missing Action")
	}
}

func TestSlackFormatter_FormatBatch_NoRxPower(t *testing.T) {
	f := &SlackFormatter{}
	events := []model.TrapEvent{{Board: 1, PON: 1, OnuID: 1, EventType: "LOS", Name: "Test"}}
	data, _ := f.FormatBatch(SeverityCritical, events)
	if strings.Contains(string(data), "RX Power") {
		t.Error("should not contain RX Power when empty")
	}
}

func TestSlackFormatter_FormatBatch_NoAction(t *testing.T) {
	f := &SlackFormatter{}
	events := []model.TrapEvent{{Board: 1, PON: 1, OnuID: 1, EventType: "Unknown"}}
	data, _ := f.FormatBatch(SeverityUnknown, events)
	if strings.Contains(string(data), "Action") {
		t.Error("unknown severity should not have Action")
	}
}

func TestSlackFormatter_FormatBatch_MultipleSeparated(t *testing.T) {
	f := &SlackFormatter{}
	events := []model.TrapEvent{
		{Board: 1, PON: 1, OnuID: 1, EventType: "LOS", Name: "A", RXPower: "-20.00"},
		{Board: 1, PON: 1, OnuID: 2, EventType: "LOS", Name: "B"},
	}
	data, _ := f.FormatBatch(SeverityCritical, events)
	if !strings.Contains(string(data), "===") {
		t.Error("missing separator between events")
	}
}

func TestGenericFormatter_FormatBatch(t *testing.T) {
	f := &GenericFormatter{}
	data, err := f.FormatBatch(SeverityCritical, batchEvents)
	if err != nil {
		t.Fatalf("FormatBatch error = %v", err)
	}

	var result []model.TrapEvent
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 events, got %d", len(result))
	}
}
