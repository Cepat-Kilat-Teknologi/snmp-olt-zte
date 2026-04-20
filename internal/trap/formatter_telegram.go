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
	b.WriteString(fmt.Sprintf("<b>%s</b>\n\n", eventTitle(event)))
	b.WriteString(fmt.Sprintf("<b>Nama:</b> %s\n", fieldOrDash(event.Name)))
	b.WriteString(fmt.Sprintf("<b>Event:</b> %s\n", fieldOrDash(event.EventType)))
	b.WriteString(fmt.Sprintf("<b>Last Offline:</b> %s\n", formatTimestampWIB(ts)))
	b.WriteString(fmt.Sprintf("<b>Board/PON/ONU:</b> %d/%d/%d\n", event.Board, event.PON, event.OnuID))

	if event.SerialNumber != "" {
		b.WriteString(fmt.Sprintf("<b>Serial:</b> %s\n", event.SerialNumber))
	}
	if event.OnuType != "" {
		b.WriteString(fmt.Sprintf("<b>ONU Type:</b> %s\n", event.OnuType))
	}
	if event.RXPower != "" {
		b.WriteString(fmt.Sprintf("<b>RX Power:</b> %s dBm\n", event.RXPower))
	}
	if event.Description != "" {
		b.WriteString(fmt.Sprintf("<b>Alamat:</b> %s\n", event.Description))
	}

	action := severityAction(sev)
	if action != "" {
		b.WriteString(fmt.Sprintf("\n\u26A0\uFE0F <b>Action:</b> %s\n", action))
	}

	b.WriteString(fmt.Sprintf("\n<code>%s</code>", formatTimestampWIB(ts)))

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
	b.WriteString(fmt.Sprintf("<b>%s %s - %d ONU %s</b>\n\n", emoji, label, len(events), category))

	for i, e := range events {
		b.WriteString(fmt.Sprintf("<b>Full Name</b> : %s\n", fieldOrDash(e.Name)))
		b.WriteString(fmt.Sprintf("<b>Address</b> : %s\n", fieldOrDash(e.Description)))
		b.WriteString(fmt.Sprintf("<b>Event</b> : %s\n", e.EventType))
		b.WriteString(fmt.Sprintf("<b>Board/PON/ONU</b> : %d/%d/%d\n", e.Board, e.PON, e.OnuID))
		if e.RXPower != "" {
			b.WriteString(fmt.Sprintf("<b>RX Power</b> : %s dBm\n", e.RXPower))
		}
		b.WriteString(fmt.Sprintf("<b>Last Online</b> : %s\n", formatLastOnline(e)))
		if i < len(events)-1 {
			b.WriteString("\n========================\n\n")
		}
	}

	action := severityAction(sev)
	if action != "" {
		b.WriteString(fmt.Sprintf("\n\n\u26A0\uFE0F <b>Action</b>\n%s", action))
	}

	b.WriteString(fmt.Sprintf("\n\n<code>%s</code>", formatTimestampWIB(ts)))

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
