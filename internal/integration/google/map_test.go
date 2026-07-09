package google_test

import (
	"testing"
	"time"

	"github.com/dylanbr0wn/shiet/internal/integration/google"
	"github.com/dylanbr0wn/shiet/internal/service"
)

func TestMapEventTimed(t *testing.T) {
	inc, err := google.MapEventForTest(10, google.EventForTest{
		ID:      "evt-1",
		ICalUID: "uid@google.com",
		Summary: "Standup",
		Start:   google.EventTimeForTest{DateTime: "2026-06-02T09:00:00-04:00", TimeZone: "America/Toronto"},
		End:     google.EventTimeForTest{DateTime: "2026-06-02T09:30:00-04:00", TimeZone: "America/Toronto"},
		Organizer: &google.PersonForTest{DisplayName: "Lead"},
		Attendees: []google.AttendeeForTest{
			{Email: "me@example.com", Self: true, ResponseStatus: "needsAction"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if inc.CalendarID != 10 || inc.Provider != service.ProviderGoogle {
		t.Fatalf("identity: %+v", inc)
	}
	if inc.Status != "needsAction" || inc.Organizer != "Lead" {
		t.Fatalf("status/organizer: %+v", inc)
	}
	if inc.Start == nil || !inc.Start.Equal(time.Date(2026, 6, 2, 13, 0, 0, 0, time.UTC)) {
		t.Fatalf("start: %+v", inc.Start)
	}
}

func TestMapEventAllDay(t *testing.T) {
	inc, err := google.MapEventForTest(1, google.EventForTest{
		ID:      "evt-day",
		Summary: "Offsite",
		Start:   google.EventTimeForTest{Date: "2026-06-03", TimeZone: "America/Toronto"},
		End:     google.EventTimeForTest{Date: "2026-06-04", TimeZone: "America/Toronto"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !inc.AllDay || inc.StartDate != "2026-06-03" || inc.EndDate != "2026-06-04" {
		t.Fatalf("all-day: %+v", inc)
	}
	if inc.OriginalTz != "America/Toronto" {
		t.Fatalf("tz: %q", inc.OriginalTz)
	}
}

func TestMapEventRecurringInstanceID(t *testing.T) {
	inc, err := google.MapEventForTest(1, google.EventForTest{
		ID:                "inst-1",
		RecurringEventID:  "series-1",
		Start:             google.EventTimeForTest{DateTime: "2026-06-02T10:00:00Z"},
		End:               google.EventTimeForTest{DateTime: "2026-06-02T11:00:00Z"},
		OriginalStartTime: &google.EventTimeForTest{DateTime: "2026-06-02T09:00:00Z"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if inc.InstanceID != "2026-06-02T09:00:00Z" {
		t.Fatalf("instance id: %q", inc.InstanceID)
	}
}

func TestMapEventCancelledSkipped(t *testing.T) {
	_, err := google.MapEventForTest(1, google.EventForTest{
		ID:     "gone",
		Status: "cancelled",
		Start:  google.EventTimeForTest{DateTime: "2026-06-02T10:00:00Z"},
		End:    google.EventTimeForTest{DateTime: "2026-06-02T11:00:00Z"},
	})
	if err == nil {
		t.Fatal("expected cancelled error")
	}
}
