package appapi_test

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"
	appv1 "github.com/dylanbr0wn/shiet/gen/shiet/app/v1"
	"github.com/dylanbr0wn/shiet/gen/shiet/app/v1/appv1connect"
	"github.com/dylanbr0wn/shiet/internal/api/appapi"
	"github.com/dylanbr0wn/shiet/internal/db"
	"github.com/dylanbr0wn/shiet/internal/seed"
	"github.com/dylanbr0wn/shiet/internal/service"
)

func TestConvertAllDayEventWire(t *testing.T) {
	t.Parallel()
	conn, err := db.Open(filepath.Join(t.TempDir(), "shiet.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(conn); err != nil {
		t.Fatal(err)
	}
	if err := seed.Dev(context.Background(), conn); err != nil {
		t.Fatal(err)
	}

	svc := service.New(conn)
	httpClient := &http.Client{Transport: handlerTransport{handler: appapi.NewHandler(appapi.Dependencies{Service: svc})}}
	periodClient := appv1connect.NewPeriodServiceClient(httpClient, "http://shiet.test")
	scheduleClient := appv1connect.NewScheduleServiceClient(httpClient, "http://shiet.test")
	calendarClient := appv1connect.NewCalendarServiceClient(httpClient, "http://shiet.test")

	periods, err := periodClient.ListPeriods(context.Background(), connect.NewRequest(&appv1.ListPeriodsRequest{}))
	if err != nil || len(periods.Msg.Periods) == 0 {
		t.Fatalf("list periods: %#v err=%v", periods, err)
	}
	periodID := periods.Msg.Periods[0].Id

	cals, err := calendarClient.ListCalendars(context.Background(), connect.NewRequest(&appv1.ListCalendarsRequest{}))
	if err != nil || len(cals.Msg.Calendars) == 0 {
		t.Fatalf("list calendars: %#v err=%v", cals, err)
	}
	calID := cals.Msg.Calendars[0].Id

	if _, err := svc.SyncEvents(context.Background(), periodID, []service.IncomingEvent{{
		CalendarID: calID,
		Provider:   service.ProviderGoogle,
		ExternalID: "wire-allday",
		Title:      "Wire Holiday",
		Status:     "accepted",
		AllDay:     true,
		StartDate:  "2026-06-03",
		EndDate:    "2026-06-04",
	}}); err != nil {
		t.Fatal(err)
	}
	events, err := scheduleClient.ListEvents(context.Background(), connect.NewRequest(&appv1.ListEventsRequest{PeriodId: periodID}))
	if err != nil || len(events.Msg.Events) != 1 {
		t.Fatalf("list events: %#v err=%v", events, err)
	}
	eventID := events.Msg.Events[0].Id

	converted, err := scheduleClient.ConvertAllDayEvent(context.Background(), connect.NewRequest(&appv1.ConvertAllDayEventRequest{
		EventId: eventID,
		Input: &appv1.TimeEntryInput{
			PeriodId:     periodID,
			Day:          "2026-06-03",
			StartMinutes: 9 * 60,
			EndMinutes:   12 * 60,
		},
	}))
	if err != nil || converted.Msg.TimeEntry == nil {
		t.Fatalf("convert: %#v err=%v", converted, err)
	}
	te := converted.Msg.TimeEntry
	if te.Attestation != "draft" || te.Method != "calendar_convert" {
		t.Fatalf("want draft calendar_convert, got attestation=%q method=%q", te.Attestation, te.Method)
	}
	if te.DurationMinutes != 180 || te.Description != "Wire Holiday" {
		t.Fatalf("payload: %#v", te)
	}

	_, err = scheduleClient.ConvertAllDayEvent(context.Background(), connect.NewRequest(&appv1.ConvertAllDayEventRequest{
		EventId: eventID,
		Input: &appv1.TimeEntryInput{
			PeriodId:     periodID,
			Day:          "2026-06-03",
			StartMinutes: 13 * 60,
			EndMinutes:   14 * 60,
		},
	}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("duplicate convert code = %v", connect.CodeOf(err))
	}
}
