package trap

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/model"
)

type telegramPayload struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
}

// TelegramFormatter formats TrapEvent as Telegram HTML messages.
type TelegramFormatter struct {
	chatID string
}

func (f *TelegramFormatter) Format(event model.TrapEvent) ([]byte, error) {
	ts := event.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}

	sev := eventSeverity(event.EventType)

	var b strings.Builder
	fmt.Fprintf(&b, "<b>%s</b>\n\n", eventTitle(event))
	fmt.Fprintf(&b, "<b>Name:</b> %s\n", fieldOrDash(event.Name))
	fmt.Fprintf(&b, "<b>Event:</b> %s\n", fieldOrDash(event.EventType))
	fmt.Fprintf(&b, "<b>Last Offline:</b> %s\n", formatTimestampWIB(ts))
	fmt.Fprintf(&b, "<b>Board/PON/ONU:</b> %d/%d/%d\n", event.Board, event.PON, event.OnuID)

	if event.SerialNumber != "" {
		fmt.Fprintf(&b, "<b>Serial:</b> %s\n", event.SerialNumber)
	}
	if event.OnuType != "" {
		fmt.Fprintf(&b, "<b>ONU Type:</b> %s\n", event.OnuType)
	}
	if event.RXPower != "" {
		fmt.Fprintf(&b, "<b>RX Power:</b> %s dBm\n", event.RXPower)
	}
	if event.Description != "" {
		fmt.Fprintf(&b, "<b>Address:</b> %s\n", event.Description)
	}

	action := severityAction(sev)
	if action != "" {
		fmt.Fprintf(&b, "\n\u26A0\uFE0F <b>Action:</b> %s\n", action)
	}

	fmt.Fprintf(&b, "\n<code>%s</code>", formatTimestampWIB(ts))

	payload := telegramPayload{
		ChatID:    f.chatID,
		Text:      b.String(),
		ParseMode: "HTML",
	}

	return json.Marshal(payload)
}

func (f *TelegramFormatter) FormatBatch(sev Severity, events []model.TrapEvent) ([]byte, error) {
	ts := time.Now()
	emoji := severityEmoji(sev)
	label := severityLabel(sev)

	category := batchCategory(sev, events)

	var b strings.Builder
	fmt.Fprintf(&b, "<b>%s %s - %d ONU %s</b>\n\n", emoji, label, len(events), category)

	for i, e := range events {
		fmt.Fprintf(&b, "<b>Full Name</b> : %s\n", fieldOrDash(e.Name))
		fmt.Fprintf(&b, "<b>Address</b> : %s\n", fieldOrDash(e.Description))
		fmt.Fprintf(&b, "<b>Event</b> : %s\n", e.EventType)
		fmt.Fprintf(&b, "<b>Board/PON/ONU</b> : %d/%d/%d\n", e.Board, e.PON, e.OnuID)
		if e.RXPower != "" {
			fmt.Fprintf(&b, "<b>RX Power</b> : %s dBm\n", e.RXPower)
		}
		fmt.Fprintf(&b, "<b>Last Online</b> : %s\n", formatLastOnline(e))
		if i < len(events)-1 {
			b.WriteString("\n========================\n\n")
		}
	}

	action := severityAction(sev)
	if action != "" {
		fmt.Fprintf(&b, "\n\n\u26A0\uFE0F <b>Action</b>\n%s", action)
	}

	fmt.Fprintf(&b, "\n\n<code>%s</code>", formatTimestampWIB(ts))

	payload := telegramPayload{
		ChatID:    f.chatID,
		Text:      b.String(),
		ParseMode: "HTML",
	}

	return json.Marshal(payload)
}

func (f *TelegramFormatter) ContentType() string {
	return "application/json"
}
