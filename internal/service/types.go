package service

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/dylanbr0wn/shiet/internal/db/sqlc"
)

// Domain types are the clean, frontend-facing shapes the service returns.
// They replace sqlc's sql.Null* columns with Go-native zero values / pointers,
// parse stored RFC3339 timestamps into time.Time, and decode JSON columns
// (attendees) into structs — so the Wails layer binds ergonomic types.

// Category is a user-defined time bucket.
type Category struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Key          string `json:"key"`
	Color        string `json:"color"`
	IsDefaultGap bool   `json:"isDefaultGap"`
	Archived     bool   `json:"archived"`
	InUse        bool   `json:"inUse"`
}

// EventCategoryOverlay is a category decision attached to an imported event.
type EventCategoryOverlay struct {
	Provider   string `json:"provider"`
	ExternalID string `json:"externalId"`
	InstanceID string `json:"instanceId,omitempty"`
	CategoryID int64  `json:"categoryId"`
}

// Period is a live, editable pay-period working record.
type Period struct {
	ID                int64      `json:"id"`
	StartDate         string     `json:"startDate"` // YYYY-MM-DD, inclusive
	EndDate           string     `json:"endDate"`   // YYYY-MM-DD, inclusive
	Cadence           string     `json:"cadence"`
	AnchorDate        string     `json:"anchorDate"`
	TargetHoursPerDay float64    `json:"targetHoursPerDay"`
	LastSyncedAt      *time.Time `json:"lastSyncedAt,omitempty"`
}

// TzSegment is a date-anchored timezone segment within a period.
type TzSegment struct {
	ID                int64  `json:"id"`
	PeriodID          int64  `json:"periodId"`
	EffectiveFromDate string `json:"effectiveFromDate"` // YYYY-MM-DD
	IanaTz            string `json:"ianaTz"`
}

// Calendar is a synced calendar source in the account-level selectable scope.
type Calendar struct {
	ID                int64  `json:"id"`
	Provider          string `json:"provider"`
	ExternalID        string `json:"externalId"`
	Name              string `json:"name"`
	IsPrimary         bool   `json:"isPrimary"`
	Selected          bool   `json:"selected"`
	DefaultCategoryID *int64 `json:"defaultCategoryId,omitempty"`
}

// GitHubRepo is a synced GitHub repository available as an evidence source.
type GitHubRepo struct {
	ID         int64  `json:"id"`
	AccountID  string `json:"accountId"`
	ExternalID string `json:"externalId"`
	Name       string `json:"name"`
	FullName   string `json:"fullName"`
	Private    bool   `json:"private"`
	Selected   bool   `json:"selected"`
}

// SlackChannel is a synced Slack channel available as an evidence source.
type SlackChannel struct {
	ID         int64  `json:"id"`
	AccountID  string `json:"accountId"`
	ExternalID string `json:"externalId"`
	Name       string `json:"name"`
	Private    bool   `json:"private"`
	Selected   bool   `json:"selected"`
}

// BitbucketWorkspace is a synced Bitbucket workspace available as an evidence source.
type BitbucketWorkspace struct {
	ID         int64  `json:"id"`
	AccountID  string `json:"accountId"`
	ExternalID string `json:"externalId"`
	Slug       string `json:"slug"`
	Name       string `json:"name"`
	Selected   bool   `json:"selected"`
}

// BitbucketRepo is a synced Bitbucket repository available as an evidence source.
type BitbucketRepo struct {
	ID            int64  `json:"id"`
	AccountID     string `json:"accountId"`
	WorkspaceUUID string `json:"workspaceUuid"`
	ExternalID    string `json:"externalId"`
	Name          string `json:"name"`
	FullName      string `json:"fullName"`
	Private       bool   `json:"private"`
	Selected      bool   `json:"selected"`
}

// Attendee mirrors the fields shiet keeps from a Google Calendar attendee.
type Attendee struct {
	Email          string `json:"email"`
	DisplayName    string `json:"displayName,omitempty"`
	ResponseStatus string `json:"responseStatus,omitempty"` // accepted | declined | tentative | needsAction
	Organizer      bool   `json:"organizer,omitempty"`
	Self           bool   `json:"self,omitempty"`
}

// Event is a synced calendar fact. For timed events Start/End are set; for
// all-day events StartDate/EndDate (date-only) are set instead.
type Event struct {
	ID               int64      `json:"id"`
	PeriodID         int64      `json:"periodId"`
	CalendarID       int64      `json:"calendarId"`
	Provider         string     `json:"provider"`
	ExternalID       string     `json:"externalId"`
	InstanceID       string     `json:"instanceId,omitempty"`
	RecurringEventID string     `json:"recurringEventId,omitempty"`
	ICalUID          string     `json:"icalUid,omitempty"`
	Title            string     `json:"title"`
	Description      string     `json:"description,omitempty"`
	Location         string     `json:"location,omitempty"`
	Organizer        string     `json:"organizer,omitempty"`
	Attendees        []Attendee `json:"attendees"`
	Status           string     `json:"status,omitempty"`
	AllDay           bool       `json:"allDay"`
	Start            *time.Time `json:"start,omitempty"`
	End              *time.Time `json:"end,omitempty"`
	StartDate        string     `json:"startDate,omitempty"` // all-day only
	EndDate          string     `json:"endDate,omitempty"`   // all-day only
	OriginalTz       string     `json:"originalTz,omitempty"`
	Active           bool       `json:"active"`
}

// ReviewItem is a sync/dedup conflict awaiting explicit user resolution.
type ReviewItem struct {
	ID              int64  `json:"id"`
	PeriodID        int64  `json:"periodId"`
	Kind            string `json:"kind"`
	EventID         *int64 `json:"eventId,omitempty"`
	Payload         string `json:"payload"` // raw JSON context
	Status          string `json:"status"`
	ConflictKey     string `json:"conflictKey,omitempty"`
	DecisionAction  string `json:"decisionAction,omitempty"`
	DecisionPayload string `json:"decisionPayload,omitempty"`
}

// ReviewDecisionAction is one allowed user choice for a review decision.
type ReviewDecisionAction struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Role    string `json:"role"`
	Variant string `json:"variant,omitempty"`
}

// ReviewDecision is the frontend-facing review read model.
type ReviewDecision struct {
	ID          int64                  `json:"id"`
	Kind        string                 `json:"kind"`
	EventID     *int64                 `json:"eventId,omitempty"`
	Tag         string                 `json:"tag"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Actions     []ReviewDecisionAction `json:"actions"`
}

// GapFill is a user entry covering an uncovered interval / manual block.
type GapFill struct {
	ID          int64  `json:"id"`
	PeriodID    int64  `json:"periodId"`
	Day         string `json:"day"` // YYYY-MM-DD
	Start       string `json:"start"`
	End         string `json:"end"`
	CategoryID  *int64 `json:"categoryId,omitempty"`
	Note        string `json:"note,omitempty"`
	Description string `json:"description,omitempty"`
	Source      string `json:"source"`
}

// ── converters from sqlc rows ─────────────────────────────────────────

func toCategory(r sqlc.Category) Category {
	return Category{
		ID:           r.ID,
		Name:         r.Name,
		Description:  r.Description,
		Key:          r.Key,
		Color:        r.Color,
		IsDefaultGap: r.IsDefaultGap != 0,
		Archived:     r.ArchivedAt.Valid,
	}
}

func toPeriod(r sqlc.Period) Period {
	return Period{
		ID:                r.ID,
		StartDate:         r.StartDate,
		EndDate:           r.EndDate,
		Cadence:           r.Cadence,
		AnchorDate:        r.AnchorDate,
		TargetHoursPerDay: r.TargetHoursPerDay,
		LastSyncedAt:      parseTimePtr(r.LastSyncedAt),
	}
}

func toTzSegment(r sqlc.TzSegment) TzSegment {
	return TzSegment{ID: r.ID, PeriodID: r.PeriodID, EffectiveFromDate: r.EffectiveFromDate, IanaTz: r.IanaTz}
}

func toGitHubRepo(r sqlc.GithubRepo) GitHubRepo {
	return GitHubRepo{
		ID:         r.ID,
		AccountID:  r.AccountID,
		ExternalID: r.ExternalID,
		Name:       r.Name,
		FullName:   r.FullName,
		Private:    r.Private == 1,
		Selected:   r.Selected == 1,
	}
}

func toSlackChannel(r sqlc.SlackChannel) SlackChannel {
	return SlackChannel{
		ID:         r.ID,
		AccountID:  r.AccountID,
		ExternalID: r.ExternalID,
		Name:       r.Name,
		Private:    r.IsPrivate == 1,
		Selected:   r.Selected == 1,
	}
}

func toBitbucketWorkspace(r sqlc.BitbucketWorkspace) BitbucketWorkspace {
	return BitbucketWorkspace{
		ID:         r.ID,
		AccountID:  r.AccountID,
		ExternalID: r.ExternalID,
		Slug:       r.Slug,
		Name:       r.Name,
		Selected:   r.Selected == 1,
	}
}

func toBitbucketRepo(r sqlc.BitbucketRepo) BitbucketRepo {
	return BitbucketRepo{
		ID:            r.ID,
		AccountID:     r.AccountID,
		WorkspaceUUID: r.WorkspaceUuid,
		ExternalID:    r.ExternalID,
		Name:          r.Name,
		FullName:      r.FullName,
		Private:       r.Private == 1,
		Selected:      r.Selected == 1,
	}
}

func toCalendar(r sqlc.Calendar) Calendar {
	return Calendar{
		ID:                r.ID,
		Provider:          r.Provider,
		ExternalID:        r.ExternalID,
		Name:              r.Name,
		IsPrimary:         r.IsPrimary != 0,
		Selected:          r.Selected != 0,
		DefaultCategoryID: nullInt64Ptr(r.DefaultCategoryID),
	}
}

func toEvent(r sqlc.Event) Event {
	return Event{
		ID:               r.ID,
		PeriodID:         r.PeriodID,
		CalendarID:       r.CalendarID,
		Provider:         r.Provider,
		ExternalID:       r.ExternalID,
		InstanceID:       r.InstanceID,
		RecurringEventID: r.RecurringEventID,
		ICalUID:          r.IcalUid,
		Title:            r.Title,
		Description:      r.Description,
		Location:         r.Location,
		Organizer:        r.Organizer,
		Attendees:        parseAttendees(r.Attendees),
		Status:           r.Status,
		AllDay:           r.AllDay != 0,
		Start:            parseTimePtr(r.StartUtc),
		End:              parseTimePtr(r.EndUtc),
		StartDate:        r.StartDate.String,
		EndDate:          r.EndDate.String,
		OriginalTz:       r.OriginalTz,
		Active:           r.Active != 0,
	}
}

func toReviewItem(r sqlc.ReviewItem) ReviewItem {
	return ReviewItem{
		ID:              r.ID,
		PeriodID:        r.PeriodID,
		Kind:            r.Kind,
		EventID:         nullInt64Ptr(r.EventID),
		Payload:         r.Payload,
		Status:          r.Status,
		ConflictKey:     r.ConflictKey,
		DecisionAction:  r.DecisionAction,
		DecisionPayload: r.DecisionPayload,
	}
}

func toGapFill(r sqlc.TimeEntry) GapFill {
	// Bridge: GapFill API remains until Connect/UI rewire. Source is derived from
	// optional provenance method; note mirrors description (note column dropped).
	source := "manual"
	if r.Method.Valid && r.Method.String == "gap_fill" {
		source = "gap"
	}
	return GapFill{
		ID:          r.ID,
		PeriodID:    r.PeriodID,
		Day:         r.LocalWorkDate,
		Start:       r.StartInstant,
		End:         r.EndInstant,
		CategoryID:  nullInt64Ptr(r.CategoryID),
		Note:        r.Description,
		Description: r.Description,
		Source:      source,
	}
}

// ── scalar helpers ────────────────────────────────────────────────────

func nullInt64Ptr(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	v := n.Int64
	return &v
}

// parseTime parses a stored RFC3339 timestamp; returns zero time on failure.
func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

// parseTimePtr parses a nullable stored timestamp into a *time.Time (nil when
// the column is NULL or empty).
func parseTimePtr(n sql.NullString) *time.Time {
	if !n.Valid || n.String == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, n.String)
	if err != nil {
		return nil
	}
	t = t.UTC()
	return &t
}

// parseAttendees decodes the JSON attendees column; always returns non-nil.
func parseAttendees(s string) []Attendee {
	out := []Attendee{}
	if s == "" || s == "[]" {
		return out
	}
	_ = json.Unmarshal([]byte(s), &out)
	return out
}
