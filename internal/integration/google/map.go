package google

import (
	"fmt"
	"strings"
	"time"

	"github.com/dylanbr0wn/shiet/internal/service"
)

// mapEvent converts a Google Calendar API event into a shiet IncomingEvent.
func mapEvent(calendarID int64, ev event) (service.IncomingEvent, error) {
	if ev.Status == "cancelled" {
		return service.IncomingEvent{}, errCancelled
	}

	inc := service.IncomingEvent{
		CalendarID:       calendarID,
		Provider:         service.ProviderGoogle,
		ExternalID:       ev.ID,
		RecurringEventID: ev.RecurringEventID,
		ICalUID:          ev.ICalUID,
		Title:            ev.Summary,
		Description:      ev.Description,
		Location:         ev.Location,
		Organizer:        organizerLabel(ev.Organizer),
		Attendees:        mapAttendees(ev.Attendees),
		Status:           selfResponseStatus(ev.Attendees),
	}

	if ev.RecurringEventID != "" {
		inc.InstanceID = instanceID(ev)
	}

	if ev.Start.Date != "" {
		inc.AllDay = true
		inc.StartDate = ev.Start.Date
		inc.EndDate = ev.End.Date
		if tz := strings.TrimSpace(ev.Start.TimeZone); tz != "" {
			inc.OriginalTz = tz
		}
		return inc, nil
	}

	start, err := parseDateTime(ev.Start.DateTime)
	if err != nil {
		return service.IncomingEvent{}, fmt.Errorf("parse start %q: %w", ev.Start.DateTime, err)
	}
	end, err := parseDateTime(ev.End.DateTime)
	if err != nil {
		return service.IncomingEvent{}, fmt.Errorf("parse end %q: %w", ev.End.DateTime, err)
	}

	inc.Start = &start
	inc.End = &end
	if tz := strings.TrimSpace(ev.Start.TimeZone); tz != "" {
		inc.OriginalTz = tz
	} else if tz := strings.TrimSpace(ev.End.TimeZone); tz != "" {
		inc.OriginalTz = tz
	}
	return inc, nil
}

// instanceID returns the stable occurrence key for a recurring instance.
func instanceID(ev event) string {
	if ev.OriginalStartTime != nil {
		if ev.OriginalStartTime.Date != "" {
			return ev.OriginalStartTime.Date
		}
		if ev.OriginalStartTime.DateTime != "" {
			if t, err := parseDateTime(ev.OriginalStartTime.DateTime); err == nil {
				return t.UTC().Format(time.RFC3339)
			}
			return ev.OriginalStartTime.DateTime
		}
	}
	if ev.Start.Date != "" {
		return ev.Start.Date
	}
	if ev.Start.DateTime != "" {
		if t, err := parseDateTime(ev.Start.DateTime); err == nil {
			return t.UTC().Format(time.RFC3339)
		}
		return ev.Start.DateTime
	}
	return ""
}

func mapAttendees(in []attendee) []service.Attendee {
	if len(in) == 0 {
		return nil
	}
	out := make([]service.Attendee, len(in))
	for i, a := range in {
		out[i] = service.Attendee{
			Email:          a.Email,
			DisplayName:    a.DisplayName,
			ResponseStatus: a.ResponseStatus,
			Organizer:      a.Organizer,
			Self:           a.Self,
		}
	}
	return out
}

func selfResponseStatus(attendees []attendee) string {
	for _, a := range attendees {
		if a.Self {
			return a.ResponseStatus
		}
	}
	return ""
}

func organizerLabel(p *person) string {
	if p == nil {
		return ""
	}
	if name := strings.TrimSpace(p.DisplayName); name != "" {
		return name
	}
	return strings.TrimSpace(p.Email)
}

func parseDateTime(raw string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}
