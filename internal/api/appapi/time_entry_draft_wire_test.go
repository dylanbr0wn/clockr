package appapi_test

import (
	"context"
	"database/sql"
	"net/http"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"
	appv1 "github.com/dylanbr0wn/shiet/gen/shiet/app/v1"
	"github.com/dylanbr0wn/shiet/gen/shiet/app/v1/appv1connect"
	"github.com/dylanbr0wn/shiet/internal/api/appapi"
	"github.com/dylanbr0wn/shiet/internal/db"
	"github.com/dylanbr0wn/shiet/internal/db/sqlc"
	"github.com/dylanbr0wn/shiet/internal/seed"
	"github.com/dylanbr0wn/shiet/internal/service"
)

func TestConfirmRejectSplitAdjustDraftWire(t *testing.T) {
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

	q := sqlc.New(conn)
	httpClient := &http.Client{Transport: handlerTransport{handler: appapi.NewHandler(appapi.Dependencies{Service: service.New(conn)})}}
	periodClient := appv1connect.NewPeriodServiceClient(httpClient, "http://shiet.test")
	scheduleClient := appv1connect.NewScheduleServiceClient(httpClient, "http://shiet.test")

	periods, err := periodClient.ListPeriods(context.Background(), connect.NewRequest(&appv1.ListPeriodsRequest{}))
	if err != nil || len(periods.Msg.Periods) == 0 {
		t.Fatalf("list periods: %#v err=%v", periods, err)
	}
	periodID := periods.Msg.Periods[0].Id

	draft, err := q.CreateTimeEntry(context.Background(), sqlc.CreateTimeEntryParams{
		PeriodID:        periodID,
		StartInstant:    "2026-06-01T13:00:00Z",
		EndInstant:      "2026-06-01T17:00:00Z",
		DurationMinutes: 240,
		LocalWorkDate:   "2026-06-01",
		Description:     "Wire draft",
		Attestation:     "draft",
		WorkType:        "worked",
		BillableStatus:  "unset",
	})
	if err != nil {
		t.Fatal(err)
	}

	adjusted, err := scheduleClient.AdjustDraftTimeEntry(context.Background(), connect.NewRequest(&appv1.AdjustDraftTimeEntryRequest{
		Id: draft.ID,
		Input: &appv1.TimeEntryInput{
			PeriodId:     periodID,
			Day:          "2026-06-01",
			StartMinutes: 9 * 60,
			EndMinutes:   12 * 60,
			Description:  "Adjusted",
		},
	}))
	if err != nil || adjusted.Msg.TimeEntry == nil || adjusted.Msg.TimeEntry.Attestation != "draft" {
		t.Fatalf("adjust: %#v err=%v", adjusted, err)
	}
	if adjusted.Msg.TimeEntry.DurationMinutes != 180 || adjusted.Msg.TimeEntry.Description != "Adjusted" {
		t.Fatalf("adjust payload: %#v", adjusted.Msg.TimeEntry)
	}

	split, err := scheduleClient.SplitTimeEntry(context.Background(), connect.NewRequest(&appv1.SplitTimeEntryRequest{
		Id:        draft.ID,
		PeriodId:  periodID,
		CutPoints: []string{"2026-06-01T14:30:00Z"},
	}))
	if err != nil || len(split.Msg.TimeEntries) != 2 {
		t.Fatalf("split: %#v err=%v", split, err)
	}
	if split.Msg.TimeEntries[0].Attestation != "draft" || split.Msg.TimeEntries[1].Attestation != "draft" {
		t.Fatalf("split attestation: %#v", split.Msg.TimeEntries)
	}

	confirmed, err := scheduleClient.ConfirmTimeEntry(context.Background(), connect.NewRequest(&appv1.ConfirmTimeEntryRequest{
		Id:       split.Msg.TimeEntries[0].Id,
		PeriodId: periodID,
	}))
	if err != nil || len(confirmed.Msg.TimeEntries) != 1 || confirmed.Msg.TimeEntries[0].Attestation != "confirmed" {
		t.Fatalf("confirm: %#v err=%v", confirmed, err)
	}

	rejected, err := scheduleClient.RejectTimeEntry(context.Background(), connect.NewRequest(&appv1.RejectTimeEntryRequest{
		Id:       split.Msg.TimeEntries[1].Id,
		PeriodId: periodID,
	}))
	if err != nil || rejected.Msg.TimeEntry == nil || rejected.Msg.TimeEntry.Attestation != "dismissed" {
		t.Fatalf("reject: %#v err=%v", rejected, err)
	}

	// Confirming an already-confirmed entry is a failed precondition on the wire.
	_, err = scheduleClient.ConfirmTimeEntry(context.Background(), connect.NewRequest(&appv1.ConfirmTimeEntryRequest{
		Id:       split.Msg.TimeEntries[0].Id,
		PeriodId: periodID,
	}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("re-confirm code = %v", connect.CodeOf(err))
	}

	// Overnight without policy → invalid argument.
	overnight, err := q.CreateTimeEntry(context.Background(), sqlc.CreateTimeEntryParams{
		PeriodID:        periodID,
		StartInstant:    "2026-06-02T02:00:00Z",
		EndInstant:      "2026-06-02T06:00:00Z",
		DurationMinutes: 240,
		LocalWorkDate:   "2026-06-01",
		Description:     "Night",
		Attestation:     "draft",
		WorkType:        "worked",
		BillableStatus:  "unset",
		CategoryID:      sql.NullInt64{},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = scheduleClient.ConfirmTimeEntry(context.Background(), connect.NewRequest(&appv1.ConfirmTimeEntryRequest{
		Id:       overnight.ID,
		PeriodId: periodID,
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("overnight missing policy code = %v", connect.CodeOf(err))
	}
}
