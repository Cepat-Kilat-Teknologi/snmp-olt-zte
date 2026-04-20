package trap

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/model"
)

type discordPayload struct {
	Embeds []discordEmbed `json:"embeds"`
}

type discordEmbed struct {
	Title       string         `json:"title"`
	Description string         `json:"description,omitempty"`
	Color       int            `json:"color"`
	Fields      []discordField `json:"fields"`
	Footer      discordFooter  `json:"footer"`
	Timestamp   string         `json:"timestamp"`
}

type discordField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

type discordFooter struct {
	Text string `json:"text"`
}

// DiscordFormatter formats TrapEvent as Discord rich embeds.
type DiscordFormatter struct{}

func (f *DiscordFormatter) Format(event model.TrapEvent) ([]byte, error) {
	sev := eventSeverity(event.EventType)
	ts := event.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}

	fields := []discordField{
		{Name: "Nama", Value: fieldOrDash(event.Name), Inline: true},
		{Name: "Event", Value: fieldOrDash(event.EventType), Inline: true},
		{Name: "Last Offline", Value: formatTimestampWIB(ts), Inline: true},
		{Name: "Board/PON/ONU", Value: fmt.Sprintf("%d/%d/%d", event.Board, event.PON, event.OnuID), Inline: true},
	}

	if event.SerialNumber != "" {
		fields = append(fields, discordField{Name: "Serial Number", Value: event.SerialNumber, Inline: true})
	}
	if event.OnuType != "" {
		fields = append(fields, discordField{Name: "ONU Type", Value: event.OnuType, Inline: true})
	}
	if event.RXPower != "" {
		fields = append(fields, discordField{Name: "RX Power", Value: event.RXPower + " dBm", Inline: true})
	}
	if event.Description != "" {
		fields = append(fields, discordField{Name: "Alamat", Value: event.Description, Inline: false})
	}

	action := severityAction(sev)
	if action != "" {
		fields = append(fields, discordField{
			Name:   "\u26A0\uFE0F Action",
			Value:  action,
			Inline: false,
		})
	}

	payload := discordPayload{
		Embeds: []discordEmbed{
			{
				Title:  eventTitle(event),
				Color:  severityColorDiscord(sev),
				Fields: fields,
				Footer: discordFooter{
					Text: formatTimestampWIB(ts),
				},
				Timestamp: ts.UTC().Format(time.RFC3339),
			},
		},
	}

	return json.Marshal(payload)
}

func (f *DiscordFormatter) FormatBatch(sev Severity, events []model.TrapEvent) ([]byte, error) {
	ts := time.Now()
	emoji := severityEmoji(sev)
	label := severityLabel(sev)

	category := batchCategory(sev, events)
	title := fmt.Sprintf("%s %s - %d ONU %s", emoji, label, len(events), category)

	var desc string
	for i, e := range events {
		desc += fmt.Sprintf("**Full Name** : %s\n", fieldOrDash(e.Name))
		desc += fmt.Sprintf("**Address** : %s\n", fieldOrDash(e.Description))
		desc += fmt.Sprintf("**Event** : %s\n", e.EventType)
		desc += fmt.Sprintf("**Board/PON/ONU** : %d/%d/%d\n", e.Board, e.PON, e.OnuID)
		if e.RXPower != "" {
			desc += fmt.Sprintf("**RX Power** : %s dBm\n", e.RXPower)
		}
		desc += fmt.Sprintf("**Last Online** : %s\n", formatLastOnline(e))
		if i < len(events)-1 {
			desc += "\n> \u200B\n"
		}
	}

	action := severityAction(sev)
	if action != "" {
		desc += fmt.Sprintf("\n\n\u26A0\uFE0F **Action**\n%s", action)
	}

	payload := discordPayload{
		Embeds: []discordEmbed{
			{
				Title:       title,
				Description: desc,
				Color:       severityColorDiscord(sev),
				Footer:      discordFooter{Text: formatTimestampWIB(ts)},
				Timestamp:   ts.UTC().Format(time.RFC3339),
			},
		},
	}

	return json.Marshal(payload)
}

func (f *DiscordFormatter) ContentType() string {
	return "application/json"
}

func fieldOrDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
