package service

import (
	"encoding/json"
	"fmt"

	"github.com/dylanbr0wn/shiet/internal/db/sqlc"
)

type reviewPayload struct {
	Title  string `json:"title"`
	From   string `json:"from"`
	To     string `json:"to"`
	Reason string `json:"reason"`
	Status string `json:"status"`
}

func parseReviewPayload(raw string) reviewPayload {
	var p reviewPayload
	_ = json.Unmarshal([]byte(raw), &p)
	return p
}

func decisionEventTitle(event *sqlc.Event, payload reviewPayload) string {
	if event != nil && event.Title != "" {
		return event.Title
	}
	if payload.Title != "" {
		return payload.Title
	}
	return "Untitled event"
}

func decisionEventMinutes(event *sqlc.Event) *int {
	if event == nil {
		return nil
	}
	start := parseTimePtr(event.StartUtc)
	end := parseTimePtr(event.EndUtc)
	if start == nil || end == nil || !end.After(*start) {
		return nil
	}
	m := int(end.Sub(*start).Minutes())
	return &m
}

func formatReviewDuration(totalMinutes int) string {
	hours := totalMinutes / 60
	minutes := totalMinutes % 60
	if hours == 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	if minutes == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh %dm", hours, minutes)
}

func primaryReviewAction(key, label string) ReviewDecisionAction {
	return ReviewDecisionAction{Key: key, Label: label, Role: "primary", Variant: "default"}
}

func secondaryReviewAction(key, label string) ReviewDecisionAction {
	return ReviewDecisionAction{Key: key, Label: label, Role: "secondary", Variant: "outline"}
}

// ToDecision builds a user-facing review decision from a review_item row and
// optional event context. Returns false for unsupported kinds.
func (reviewPolicy) ToDecision(item sqlc.ReviewItem, event *sqlc.Event) (ReviewDecision, bool) {
	payload := parseReviewPayload(item.Payload)
	title := decisionEventTitle(event, payload)
	minutes := decisionEventMinutes(event)
	var eventID *int64
	if item.EventID.Valid {
		eventID = &item.EventID.Int64
	}

	switch item.Kind {
	case reviewDeletedCategoriz:
		reason := "deleted from your calendar"
		if payload.Reason == "declined" {
			reason = "declined on your calendar"
		}
		desc := fmt.Sprintf("%s but already categorized", reason)
		if minutes != nil {
			desc += fmt.Sprintf(" (%s)", formatReviewDuration(*minutes))
		}
		desc += ". Suggest dropping the entry."
		dropLabel := "Drop entry"
		if minutes != nil {
			dropLabel = fmt.Sprintf("Drop %s", formatReviewDuration(*minutes))
		}
		return ReviewDecision{
			ID:          item.ID,
			Kind:        item.Kind,
			EventID:     eventID,
			Tag:         "Removed",
			Title:       title,
			Description: desc,
			Actions: []ReviewDecisionAction{
				primaryReviewAction(ReviewActionDropEntry, dropLabel),
				secondaryReviewAction(ReviewActionKeepEntry, "Keep entry"),
			},
		}, true
	case reviewTitleChanged:
		from := payload.From
		if from == "" {
			from = "previous"
		}
		to := payload.To
		if to == "" {
			to = title
		}
		return ReviewDecision{
			ID:          item.ID,
			Kind:        item.Kind,
			EventID:     eventID,
			Tag:         "Title changed",
			Title:       title,
			Description: fmt.Sprintf(`Title changed from "%s" to "%s". Confirm the existing category still applies.`, from, to),
			Actions: []ReviewDecisionAction{
				primaryReviewAction(ReviewActionAccept, "Accept new title"),
				secondaryReviewAction(ReviewActionDismiss, "Remind me later"),
			},
		}, true
	case reviewNewInGap:
		return ReviewDecision{
			ID:          item.ID,
			Kind:        item.Kind,
			EventID:     eventID,
			Tag:         "Gap conflict",
			Title:       title,
			Description: fmt.Sprintf(`"%s" landed inside a gap you already filled. Keeping both would double-count time.`, title),
			Actions: []ReviewDecisionAction{
				primaryReviewAction(ReviewActionUseEvent, "Use event, shrink fill"),
				secondaryReviewAction(ReviewActionKeepGap, "Keep gap fill"),
			},
		}, true
	case reviewTentative:
		statusLabel := "tentative"
		if payload.Status == "needsAction" {
			statusLabel = "not responded"
		}
		return ReviewDecision{
			ID:          item.ID,
			Kind:        item.Kind,
			EventID:     eventID,
			Tag:         "Tentative",
			Title:       title,
			Description: fmt.Sprintf("Marked %s. Include it in your schedule or exclude it.", statusLabel),
			Actions: []ReviewDecisionAction{
				primaryReviewAction(ReviewActionInclude, "Include"),
				secondaryReviewAction(ReviewActionExclude, "Exclude"),
			},
		}, true
	case reviewAllDay:
		return ReviewDecision{
			ID:          item.ID,
			Kind:        item.Kind,
			EventID:     eventID,
			Tag:         "All day",
			Title:       title,
			Description: "All-day markers do not affect gap totals. Include to confirm, or dismiss the prompt.",
			Actions: []ReviewDecisionAction{
				primaryReviewAction(ReviewActionInclude, "Include"),
				secondaryReviewAction(ReviewActionExclude, "Dismiss"),
			},
		}, true
	case reviewSourceDrift:
		return ReviewDecision{
			ID:          item.ID,
			Kind:        item.Kind,
			EventID:     eventID,
			Tag:         "Source changed",
			Title:       title,
			Description: "Calendar event changed after you confirmed time. Confirmed entry stays as-is; spawn a draft proposal, replace with a new draft from the event, or dismiss.",
			Actions: []ReviewDecisionAction{
				primaryReviewAction(ReviewActionSpawnDraft, "Spawn draft"),
				secondaryReviewAction(ReviewActionReplace, "Replace with draft"),
				secondaryReviewAction(ReviewActionDismiss, "Dismiss"),
			},
		}, true
	case "overlap", "dedup_ambiguous":
		return ReviewDecision{}, false
	default:
		return ReviewDecision{}, false
	}
}
