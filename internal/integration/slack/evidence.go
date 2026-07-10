package slack

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/dylanbr0wn/shiet/internal/db/sqlc"
	"github.com/dylanbr0wn/shiet/internal/service"
)

const (
	conversationsHistoryPath = "/conversations.history"
	maxMessagesPerChannel    = 50
	summaryMaxRunes          = 120
	historyPageSize          = 200
)

// Provider returns the activity-evidence provider id.
func (p *Provider) Provider() string {
	return providerSlack
}

// FetchEvidence loads top-level messages for selected channels in window.
// Per-channel failures are skipped so one bad channel does not block the rest.
func (p *Provider) FetchEvidence(ctx context.Context, window service.TimeWindow) ([]service.ActivityEvidence, error) {
	if p.Queries == nil {
		return nil, nil
	}
	start := window.Start.UTC()
	end := window.End.UTC()
	if !end.After(start) {
		return nil, nil
	}

	channels, err := p.Queries.ListSelectedSlackChannels(ctx)
	if err != nil {
		return nil, fmt.Errorf("list selected slack channels: %w", err)
	}
	if len(channels) == 0 {
		return nil, nil
	}

	out := make([]service.ActivityEvidence, 0)
	for _, channel := range channels {
		items := p.fetchChannelEvidence(ctx, channel, start, end)
		out = append(out, items...)
	}
	return out, nil
}

func (p *Provider) fetchChannelEvidence(ctx context.Context, channel sqlc.SlackChannel, start, end time.Time) []service.ActivityEvidence {
	accountID := strings.TrimSpace(channel.AccountID)
	channelID := strings.TrimSpace(channel.ExternalID)
	name := strings.TrimSpace(channel.Name)
	if accountID == "" || channelID == "" || name == "" {
		return nil
	}

	messages, err := p.fetchChannelMessages(ctx, accountID, channelID, name, start, end)
	if err != nil {
		return nil
	}
	return messages
}

func (p *Provider) fetchChannelMessages(ctx context.Context, accountID, channelID, channelName string, start, end time.Time) ([]service.ActivityEvidence, error) {
	out := make([]service.ActivityEvidence, 0, maxMessagesPerChannel)
	cursor := ""

	for len(out) < maxMessagesPerChannel {
		q := url.Values{}
		q.Set("channel", channelID)
		q.Set("oldest", formatSlackTS(start))
		q.Set("latest", formatSlackTS(end))
		q.Set("inclusive", "true")
		q.Set("limit", strconv.Itoa(historyPageSize))
		if cursor != "" {
			q.Set("cursor", cursor)
		}

		var resp historyResponse
		if err := p.getJSON(ctx, accountID, conversationsHistoryPath, q, &resp); err != nil {
			return nil, err
		}
		if !resp.OK {
			return nil, fmt.Errorf("slack api %s: %s", conversationsHistoryPath, strings.TrimSpace(resp.Error))
		}

		if len(resp.Messages) == 0 {
			break
		}

		for _, msg := range resp.Messages {
			if len(out) >= maxMessagesPerChannel {
				break
			}
			ev, ok := mapMessage(msg, channelID, channelName, start, end)
			if !ok {
				continue
			}
			out = append(out, ev)
		}

		cursor = strings.TrimSpace(resp.ResponseMetadata.NextCursor)
		if cursor == "" {
			break
		}
	}
	return out, nil
}

func mapMessage(msg historyMessage, channelID, channelName string, start, end time.Time) (service.ActivityEvidence, bool) {
	if strings.TrimSpace(msg.Type) != "" && msg.Type != "message" {
		return service.ActivityEvidence{}, false
	}
	if strings.TrimSpace(msg.Subtype) != "" {
		return service.ActivityEvidence{}, false
	}
	if strings.TrimSpace(msg.BotID) != "" {
		return service.ActivityEvidence{}, false
	}

	ts, ok := parseSlackTS(msg.TS)
	if !ok || !inWindow(ts, start, end) {
		return service.ActivityEvidence{}, false
	}

	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return service.ActivityEvidence{}, false
	}

	summary := prefixEvidenceSummary(channelName, truncateRunes(firstLine(text), summaryMaxRunes))

	return service.ActivityEvidence{
		Provider: providerSlack,
		Kind:     "message",
		Start:    ts,
		End:      ts.Add(time.Second),
		Summary:  summary,
		Source:   channelName,
		Detail:   text,
		URL:      messagePermalink(channelID, msg.TS),
	}, true
}

func formatSlackTS(t time.Time) string {
	sec := t.Unix()
	frac := t.Nanosecond() / 1000
	return fmt.Sprintf("%d.%06d", sec, frac)
}

func parseSlackTS(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	parts := strings.SplitN(raw, ".", 2)
	sec, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	var nsec int64
	if len(parts) == 2 && parts[1] != "" {
		frac := parts[1]
		if len(frac) > 9 {
			frac = frac[:9]
		}
		for len(frac) < 9 {
			frac += "0"
		}
		nsec, err = strconv.ParseInt(frac, 10, 64)
		if err != nil {
			return time.Time{}, false
		}
	}
	return time.Unix(sec, nsec).UTC(), true
}

func inWindow(ts, start, end time.Time) bool {
	return !ts.Before(start) && ts.Before(end)
}

func firstLine(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		return strings.TrimSpace(text[:i])
	}
	return text
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

func prefixEvidenceSummary(source, summary string) string {
	source = strings.TrimSpace(source)
	summary = strings.TrimSpace(summary)
	if source == "" {
		return summary
	}
	if summary == "" {
		return source
	}
	return source + " · " + summary
}

func messagePermalink(channelID, ts string) string {
	channelID = strings.TrimSpace(channelID)
	ts = strings.TrimSpace(ts)
	if channelID == "" || ts == "" {
		return ""
	}
	return "https://slack.com/archives/" + channelID + "/p" + strings.ReplaceAll(ts, ".", "")
}

type historyResponse struct {
	OK               bool             `json:"ok"`
	Error            string           `json:"error"`
	Messages         []historyMessage `json:"messages"`
	ResponseMetadata struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
}

type historyMessage struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
	BotID   string `json:"bot_id"`
	User    string `json:"user"`
	Text    string `json:"text"`
	TS      string `json:"ts"`
}
