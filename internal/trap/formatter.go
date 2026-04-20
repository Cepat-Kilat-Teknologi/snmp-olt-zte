package trap

import (
	"fmt"
	"strings"
	"time"

	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/model"
	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/pkg/logger"
)

// WebhookFormatter formats TrapEvent into platform-specific payloads.
type WebhookFormatter interface {
	Format(event model.TrapEvent) ([]byte, error)
	FormatBatch(severity Severity, events []model.TrapEvent) ([]byte, error)
	ContentType() string
}

// Severity represents the severity level of a trap event.
type Severity int

const (
	SeverityCritical Severity = iota // LOS, LOSi, LOFi, Offline, AuthFailed, PowerOff
	SeverityHigh                     // Logging, Synchronization (stuck)
	SeverityMedium                   // HighRxPower, LowRxPower
	SeverityLow                      // DyingGasp
	SeverityUnknown
)

func eventSeverity(eventType string) Severity {
	switch eventType {
	case "LOS", "LOSi", "LOFi", "Offline", "AuthFailed", "PowerOff":
		return SeverityCritical
	case "Logging", "Synchronization":
		return SeverityHigh
	case "HighRxPower", "LowRxPower":
		return SeverityMedium
	case "DyingGasp":
		return SeverityLow
	default:
		return SeverityUnknown
	}
}

func severityEmoji(s Severity) string {
	switch s {
	case SeverityCritical:
		return "\U0001F534" // 🔴
	case SeverityHigh:
		return "\U0001F7E0" // 🟠
	case SeverityMedium:
		return "\U0001F7E1" // 🟡
	case SeverityLow:
		return "\U0001F535" // 🔵
	default:
		return "\u26AA" // ⚪
	}
}

func severityLabel(s Severity) string {
	switch s {
	case SeverityCritical:
		return "CRITICAL"
	case SeverityHigh:
		return "HIGH"
	case SeverityMedium:
		return "MEDIUM"
	case SeverityLow:
		return "LOW"
	default:
		return "UNKNOWN"
	}
}

var actionMessages = map[Severity]string{
	SeverityCritical: "Mandatory customer visit within 1x24 hours",
	SeverityHigh:     "Mandatory visit within 1x24 hours if Hard Restart does not resolve",
	SeverityMedium:   "Mandatory visit within 2x24 hours after notification",
	SeverityLow:      "Coordinate with customer to ensure no electrical issues",
}

func SetActionMessages(critical, high, medium, low string) {
	if critical != "" {
		actionMessages[SeverityCritical] = critical
	}
	if high != "" {
		actionMessages[SeverityHigh] = high
	}
	if medium != "" {
		actionMessages[SeverityMedium] = medium
	}
	if low != "" {
		actionMessages[SeverityLow] = low
	}
}

func severityAction(s Severity) string {
	return actionMessages[s]
}

func severityColorDiscord(s Severity) int {
	switch s {
	case SeverityCritical:
		return 0xFF0000 // red
	case SeverityHigh:
		return 0xFF8C00 // orange
	case SeverityMedium:
		return 0xFFD700 // yellow
	case SeverityLow:
		return 0x3498DB // blue
	default:
		return 0x95A5A6 // gray
	}
}

func severityColorHex(s Severity) string {
	switch s {
	case SeverityCritical:
		return "#FF0000"
	case SeverityHigh:
		return "#FF8C00"
	case SeverityMedium:
		return "#FFD700"
	case SeverityLow:
		return "#3498DB"
	default:
		return "#95A5A6"
	}
}

func eventTitle(event model.TrapEvent) string {
	sev := eventSeverity(event.EventType)
	emoji := severityEmoji(sev)
	label := severityLabel(sev)

	category := event.EventType
	switch sev {
	case SeverityHigh:
		category = "STUCK"
	}
	if category == "" {
		category = "Event"
	}

	return fmt.Sprintf("%s %s - ONU %s", emoji, label, category)
}

var loadLocation = time.LoadLocation

func formatTimestampWIB(t time.Time) string {
	if t.IsZero() {
		t = time.Now()
	}
	loc, err := loadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*60*60)
	}
	return t.In(loc).Format("02-01-2006 / 15:04:05") + " WIB"
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "\u2026"
}

// BuildRepeatIntervals creates a severity-to-duration map for repeat notifications (minutes).
func BuildRepeatIntervals(critical, high, medium, low int) map[Severity]time.Duration {
	m := make(map[Severity]time.Duration)
	if critical > 0 {
		m[SeverityCritical] = time.Duration(critical) * time.Minute
	}
	if high > 0 {
		m[SeverityHigh] = time.Duration(high) * time.Minute
	}
	if medium > 0 {
		m[SeverityMedium] = time.Duration(medium) * time.Minute
	}
	if low > 0 {
		m[SeverityLow] = time.Duration(low) * time.Minute
	}
	return m
}

// BuildIntervals creates a severity-to-duration map from config values (seconds).
// Zero values are omitted (that severity won't be batched).
func BuildIntervals(critical, high, medium, low int) map[Severity]time.Duration {
	m := make(map[Severity]time.Duration)
	if critical > 0 {
		m[SeverityCritical] = time.Duration(critical) * time.Second
	}
	if high > 0 {
		m[SeverityHigh] = time.Duration(high) * time.Second
	}
	if medium > 0 {
		m[SeverityMedium] = time.Duration(medium) * time.Second
	}
	if low > 0 {
		m[SeverityLow] = time.Duration(low) * time.Second
	}
	return m
}

func batchCategory(sev Severity, events []model.TrapEvent) string {
	if sev == SeverityHigh {
		return "STUCK"
	}
	if len(events) > 0 {
		return events[0].EventType
	}
	return "Event"
}

func formatLastOnline(event model.TrapEvent) string {
	raw := event.LastOnline
	if raw == "" {
		raw = event.LastOffline
	}
	if raw == "" {
		return formatTimestampWIB(event.Timestamp)
	}
	// Convert "2026-04-20 22:32:57" → "20-04-2026/22:32:57"
	t, err := time.Parse("2006-01-02 15:04:05", raw)
	if err != nil {
		return raw
	}
	return t.Format("02-01-2006/15:04:05")
}

// DetectPlatform determines the webhook platform from the URL.
func DetectPlatform(url string) string {
	switch {
	case strings.Contains(url, "discord.com/api/webhooks"):
		return "discord"
	case strings.Contains(url, "hooks.slack.com"):
		return "slack"
	case strings.Contains(url, "api.telegram.org/bot"):
		return "telegram"
	default:
		return "generic"
	}
}

func buildTelegramURL(url string) string {
	url = strings.TrimRight(url, "/")
	if strings.HasSuffix(url, "/sendMessage") {
		return url
	}
	return url + "/sendMessage"
}

// NewFormatter creates a WebhookFormatter based on platform type and URL.
// Returns the formatter and the final URL (may be modified for Telegram).
func NewFormatter(url, webhookType, chatID string) (WebhookFormatter, string) {
	platform := webhookType
	if platform == "" {
		platform = DetectPlatform(url)
	}

	switch platform {
	case "discord":
		return &DiscordFormatter{}, url
	case "slack":
		return &SlackFormatter{}, url
	case "telegram":
		if chatID == "" {
			logger.Warn("telegram_webhook_missing_chat_id_falling_back_to_generic")
			return &GenericFormatter{}, url
		}
		return &TelegramFormatter{chatID: chatID}, buildTelegramURL(url)
	default:
		return &GenericFormatter{}, url
	}
}
