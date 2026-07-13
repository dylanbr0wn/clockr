package appapi_test

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	appv1 "github.com/dylanbr0wn/shiet/gen/shiet/app/v1"
	"github.com/dylanbr0wn/shiet/gen/shiet/app/v1/appv1connect"
	"github.com/dylanbr0wn/shiet/internal/api/appapi"
	"github.com/dylanbr0wn/shiet/internal/db"
	"github.com/dylanbr0wn/shiet/internal/integration/connection"
	"github.com/dylanbr0wn/shiet/internal/seed"
	"github.com/dylanbr0wn/shiet/internal/service"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestPortableApplicationServicesShareOneConnectHandler(t *testing.T) {
	t.Parallel()
	conn, err := db.Open(filepath.Join(t.TempDir(), "shiet.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(conn); err != nil {
		t.Fatal(err)
	}
	if err := seed.Core(context.Background(), conn); err != nil {
		t.Fatal(err)
	}

	svc := service.New(conn)
	githubRefreshed := false
	handler := appapi.NewHandler(appapi.Dependencies{
		Service:    svc,
		SyncPeriod: func(context.Context, int64) (service.SyncResult, error) { return service.SyncResult{Added: 2}, nil },
		ListConnections: func(context.Context) ([]connection.Connection, error) {
			return []connection.Connection{{ID: 7, Provider: "google", AccountID: "me"}}, nil
		},
		RefreshGitHubRepos:   func(_ context.Context, accountID string) error { githubRefreshed = accountID == "octo"; return nil },
		RefreshSlackChannels: func(context.Context, string) error { return nil },
	})
	httpClient := &http.Client{Transport: handlerTransport{handler: handler}}

	categoryClient := appv1connect.NewCategoryServiceClient(httpClient, "http://shiet.test")
	created, err := categoryClient.CreateCategory(context.Background(), connect.NewRequest(&appv1.CreateCategoryRequest{Name: "Deep work", Key: "deep"}))
	if err != nil {
		t.Fatal(err)
	}
	if created.Msg.Category == nil || created.Msg.Category.Id <= 0 {
		t.Fatalf("missing created category: %#v", created.Msg)
	}
	_, err = categoryClient.CreateCategory(context.Background(), connect.NewRequest(&appv1.CreateCategoryRequest{}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("empty category code = %v", connect.CodeOf(err))
	}
	categories, err := categoryClient.ListCategories(context.Background(), connect.NewRequest(&appv1.ListCategoriesRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	for _, category := range categories.Msg.Categories {
		if category.IsDefaultGap {
			_, err = categoryClient.DeleteCategory(context.Background(), connect.NewRequest(&appv1.DeleteCategoryRequest{Id: category.Id}))
			if connect.CodeOf(err) != connect.CodeFailedPrecondition {
				t.Fatalf("delete default category code = %v", connect.CodeOf(err))
			}
			break
		}
	}

	settingsClient := appv1connect.NewSettingsServiceClient(httpClient, "http://shiet.test")
	if _, err := settingsClient.SetSetting(context.Background(), connect.NewRequest(&appv1.SetSettingRequest{Key: "test.rpc", Value: `"yes"`})); err != nil {
		t.Fatal(err)
	}
	setting, err := settingsClient.GetSetting(context.Background(), connect.NewRequest(&appv1.GetSettingRequest{Key: "test.rpc"}))
	if err != nil || setting.Msg.Value != `"yes"` {
		t.Fatalf("setting = %#v, err %v", setting, err)
	}

	calendarClient := appv1connect.NewCalendarServiceClient(httpClient, "http://shiet.test")
	syncResult, err := calendarClient.SyncPeriod(context.Background(), connect.NewRequest(&appv1.SyncPeriodRequest{PeriodId: 1}))
	if err != nil || syncResult.Msg.Added != 2 {
		t.Fatalf("sync = %#v, err %v", syncResult, err)
	}

	integrationClient := appv1connect.NewIntegrationServiceClient(httpClient, "http://shiet.test")
	connections, err := integrationClient.ListIntegrationConnections(context.Background(), connect.NewRequest(&appv1.ListIntegrationConnectionsRequest{}))
	if err != nil || len(connections.Msg.Connections) != 1 || connections.Msg.Connections[0].Id != 7 {
		t.Fatalf("connections = %#v, err %v", connections, err)
	}
	if _, err := integrationClient.RefreshGitHubRepos(context.Background(), connect.NewRequest(&appv1.RefreshGitHubReposRequest{AccountId: "octo"})); err != nil || !githubRefreshed {
		t.Fatalf("refresh github: called=%v err=%v", githubRefreshed, err)
	}

	periodClient := appv1connect.NewPeriodServiceClient(httpClient, "http://shiet.test")
	ensured, err := periodClient.EnsureCurrentPeriod(context.Background(), connect.NewRequest(&appv1.EnsureCurrentPeriodRequest{Today: "2026-07-09", IanaTz: "America/Vancouver"}))
	if err != nil || ensured.Msg.Period == nil {
		t.Fatalf("ensure period: %#v err=%v", ensured, err)
	}
	periodID := ensured.Msg.Period.Id
	scheduleClient := appv1connect.NewScheduleServiceClient(httpClient, "http://shiet.test")
	manual, err := scheduleClient.CreateManualEvent(context.Background(), connect.NewRequest(&appv1.CreateManualEventRequest{Input: &appv1.ManualEventInput{PeriodId: periodID, Day: "2026-07-09", StartMinutes: 540, EndMinutes: 600}}))
	if err != nil || manual.Msg.Id <= 0 {
		t.Fatalf("manual event: %#v err=%v", manual, err)
	}
	_, err = scheduleClient.CreateManualEvent(context.Background(), connect.NewRequest(&appv1.CreateManualEventRequest{Input: &appv1.ManualEventInput{PeriodId: periodID, Day: "2026-07-09", StartMinutes: 600, EndMinutes: 540}}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("invalid range code = %v", connect.CodeOf(err))
	}
	_, err = scheduleClient.SuggestGapFill(context.Background(), connect.NewRequest(&appv1.SuggestGapFillRequest{Start: timestamppb.New(time.Now()), End: timestamppb.New(time.Now().Add(time.Hour))}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("unconfigured AI code = %v", connect.CodeOf(err))
	}

	exportClient := appv1connect.NewExportServiceClient(httpClient, "http://shiet.test")
	templates, err := exportClient.ListExportTemplates(context.Background(), connect.NewRequest(&appv1.ListExportTemplatesRequest{}))
	if err != nil || len(templates.Msg.Templates) == 0 {
		t.Fatalf("templates = %#v, err %v", templates, err)
	}
	rendered, err := exportClient.RenderPeriodExport(context.Background(), connect.NewRequest(&appv1.RenderPeriodExportRequest{PeriodId: periodID, TemplateKey: service.ExportTemplateTextSummary}))
	if err != nil || rendered.Msg.Format != "text" {
		t.Fatalf("render = %#v err=%v", rendered, err)
	}
	_, err = exportClient.DeleteExportTemplate(context.Background(), connect.NewRequest(&appv1.DeleteExportTemplateRequest{Id: templates.Msg.Templates[0].Id}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("delete builtin code = %v", connect.CodeOf(err))
	}
	_, err = exportClient.CreateExportTemplate(context.Background(), connect.NewRequest(&appv1.CreateExportTemplateRequest{Name: "Bad", Format: "pdf", Body: "x"}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("invalid export format code = %v", connect.CodeOf(err))
	}
	_, err = exportClient.ListExportFieldCatalog(context.Background(), connect.NewRequest(&appv1.ListExportFieldCatalogRequest{Grain: "bogus", Layout: "flat"}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("invalid catalog code = %v", connect.CodeOf(err))
	}
}

func TestArchiveCategoryHidesFromDefaultList(t *testing.T) {
	t.Parallel()
	conn, err := db.Open(filepath.Join(t.TempDir(), "shiet.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(conn); err != nil {
		t.Fatal(err)
	}
	if err := seed.Core(context.Background(), conn); err != nil {
		t.Fatal(err)
	}

	client := appv1connect.NewCategoryServiceClient(
		&http.Client{Transport: handlerTransport{handler: appapi.NewHandler(appapi.Dependencies{Service: service.New(conn)})}},
		"http://shiet.test",
	)

	created, err := client.CreateCategory(context.Background(), connect.NewRequest(&appv1.CreateCategoryRequest{Name: "Archive Me"}))
	if err != nil {
		t.Fatal(err)
	}
	id := created.Msg.Category.Id

	archived, err := client.ArchiveCategory(context.Background(), connect.NewRequest(&appv1.ArchiveCategoryRequest{Id: id}))
	if err != nil {
		t.Fatal(err)
	}
	if !archived.Msg.Category.Archived {
		t.Fatal("expected archived=true")
	}

	active, err := client.ListCategories(context.Background(), connect.NewRequest(&appv1.ListCategoriesRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	for _, cat := range active.Msg.Categories {
		if cat.Id == id {
			t.Fatal("archived category still in default list")
		}
	}

	all, err := client.ListCategories(context.Background(), connect.NewRequest(&appv1.ListCategoriesRequest{IncludeArchived: true}))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, cat := range all.Msg.Categories {
		if cat.Id == id {
			found = true
			if !cat.Archived {
				t.Fatal("include_archived list missing archived flag")
			}
		}
	}
	if !found {
		t.Fatal("archived category missing from include_archived list")
	}

	got, err := client.GetCategory(context.Background(), connect.NewRequest(&appv1.GetCategoryRequest{Id: id}))
	if err != nil || !got.Msg.Category.Archived {
		t.Fatalf("get archived: %#v err=%v", got, err)
	}
}
