package trap

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/internal/model"
)

type slackPayload struct {
	Attachments []slackAttachment `json:"attachments"`
}

type slackAttachment struct {
	Color  string       `json:"color"`
	Blocks []slackBlock `json:"blocks"`
}

type slackBlock struct {
	Type   string      `json:"type"`
	Text   *slackText  `json:"text,omitempty"`
	Fields []slackText `json:"fields,omitempty"`
}

type slackText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// SlackFormatter formats TrapEvent as Slack attachments with blocks.
type SlackFormatter struct{}

func (f *SlackFormatter) Format(event model.TrapEvent) ([]byte, error) {
	sev := eventSeverity(event.EventType)
	ts := event.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}

	detailFields := []slackText{
		{Type: "mrkdwn", Text: fmt.Sprintf("*Nama:*\n%s", fieldOrDash(event.Name))},
		{Type: "mrkdwn", Text: fmt.Sprintf("*Event:*\n%s", fieldOrDash(event.EventType))},
		{Type: "mrkdwn", Text: fmt.Sprintf("*Last Offline:*\n%s", formatTimestampWIB(ts))},
		{Type: "mrkdwn", Text: fmt.Sprintf("*Board/PON/ONU:*\n%d/%d/%d", event.Board, event.PON, event.OnuID)},
	}

	if event.SerialNumber != "" {
		detailFields = append(detailFields, slackText{Type: "mrkdwn", Text: fmt.Sprintf("*Serial:*\n%s", event.SerialNumber)})
	}
	if event.OnuType != "" {
		detailFields = append(detailFields, slackText{Type: "mrkdwn", Text: fmt.Sprintf("*ONU Type:*\n%s", event.OnuType)})
	}
	if event.RXPower != "" {
		detailFields = append(detailFields, slackText{Type: "mrkdwn", Text: fmt.Sprintf("*RX Power:*\n%s dBm", event.RXPower)})
	}

	blocks := []slackBlock{
		{
			Type: "header",
			Text: &slackText{Type: "plain_text", Text: eventTitle(event)},
		},
		{
			Type:   "section",
			Fields: detailFields,
		},
	}

	if event.Description != "" {
		blocks = append(blocks, slackBlock{
			Type: "section",
			Text: &slackText{Type: "mrkdwn", Text: fmt.Sprintf("*Alamat:* %s", event.Description)},
		})
	}

	action := severityAction(sev)
	if action != "" {
		blocks = append(blocks, slackBlock{
			Type: "section",
			Text: &slackText{Type: "mrkdwn", Text: fmt.Sprintf("\u26A0\uFE0F *Action:* %s", action)},
		})
	}

	blocks = append(blocks, slackBlock{
		Type: "context",
		Text: &slackText{Type: "mrkdwn", Text: formatTimestampWIB(ts)},
	})

	payload := slackPayload{
		Attachments: []slackAttachment{
			{
				Color:  severityColorHex(sev),
				Blocks: blocks,
			},
		},
	}

	return json.Marshal(payload)
}

func (f *SlackFormatter) FormatBatch(sev Severity, events []model.TrapEvent) ([]byte, error) {
	ts := time.Now()
	emoji := severityEmoji(sev)
	label := severityLabel(sev)

	category := batchCategory(sev, events)
	title := fmt.Sprintf("%s %s - %d ONU %s", emoji, label, len(events), category)

	var body string
	for i, e := range events {
		body += fmt.Sprintf("*Full Name* : %s\n", fieldOrDash(e.Name))
		body += fmt.Sprintf("*Address* : %s\n", fieldOrDash(e.Description))
		body += fmt.Sprintf("*Event* : %s\n", e.EventType)
		body += fmt.Sprintf("*Board/PON/ONU* : %d/%d/%d\n", e.Board, e.PON, e.OnuID)
		if e.RXPower != "" {
			body += fmt.Sprintf("*RX Power* : %s dBm\n", e.RXPower)
		}
		body += fmt.Sprintf("*Last Online* : %s\n", formatLastOnline(e))
		if i < len(events)-1 {
			body += "\n========================\n\n"
		}
	}

	blocks := []slackBlock{
		{Type: "header", Text: &slackText{Type: "plain_text", Text: title}},
		{Type: "section", Text: &slackText{Type: "mrkdwn", Text: body}},
	}

	action := severityAction(sev)
	if action != "" {
		blocks = append(blocks, slackBlock{
			Type: "section",
			Text: &slackText{Type: "mrkdwn", Text: fmt.Sprintf("\u26A0\uFE0F *Action:* %s", action)},
		})
	}

	blocks = append(blocks, slackBlock{
		Type: "context",
		Text: &slackText{Type: "mrkdwn", Text: formatTimestampWIB(ts)},
	})

	payload := slackPayload{
		Attachments: []slackAttachment{{Color: severityColorHex(sev), Blocks: blocks}},
	}

	return json.Marshal(payload)
}

func (f *SlackFormatter) ContentType() string {
	return "application/json"
}
