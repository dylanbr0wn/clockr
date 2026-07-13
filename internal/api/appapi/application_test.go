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

	projectClient := appv1connect.NewProjectServiceClient(httpClient, "http://shiet.test")
	project, err := projectClient.CreateProject(context.Background(), connect.NewRequest(&appv1.CreateProjectRequest{Name: "Apollo", Key: "apollo"}))
	if err != nil || project.Msg.Project == nil || project.Msg.Project.Id <= 0 {
		t.Fatalf("create project: %#v err=%v", project, err)
	}
	_, err = projectClient.CreateProject(context.Background(), connect.NewRequest(&appv1.CreateProjectRequest{}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("empty project code = %v", connect.CodeOf(err))
	}
	projects, err := projectClient.ListProjects(context.Background(), connect.NewRequest(&appv1.ListProjectsRequest{}))
	if err != nil || len(projects.Msg.Projects) == 0 {
		t.Fatalf("list projects: %#v err=%v", projects, err)
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
	manual, err := scheduleClient.CreateTimeEntry(context.Background(), connect.NewRequest(&appv1.CreateTimeEntryRequest{Input: &appv1.TimeEntryInput{PeriodId: periodID, Day: "2026-07-09", StartMinutes: 540, EndMinutes: 600}}))
	if err != nil || manual.Msg.Id <= 0 {
		t.Fatalf("time entry: %#v err=%v", manual, err)
	}
	_, err = scheduleClient.CreateTimeEntry(context.Background(), connect.NewRequest(&appv1.CreateTimeEntryRequest{Input: &appv1.TimeEntryInput{PeriodId: periodID, Day: "2026-07-09", StartMinutes: 600, EndMinutes: 540}}))
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

func TestArchiveProjectHidesFromDefaultList(t *testing.T) {
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

	client := appv1connect.NewProjectServiceClient(
		&http.Client{Transport: handlerTransport{handler: appapi.NewHandler(appapi.Dependencies{Service: service.New(conn)})}},
		"http://shiet.test",
	)

	created, err := client.CreateProject(context.Background(), connect.NewRequest(&appv1.CreateProjectRequest{Name: "Archive Me"}))
	if err != nil {
		t.Fatal(err)
	}
	id := created.Msg.Project.Id

	archived, err := client.ArchiveProject(context.Background(), connect.NewRequest(&appv1.ArchiveProjectRequest{Id: id}))
	if err != nil {
		t.Fatal(err)
	}
	if !archived.Msg.Project.Archived {
		t.Fatal("expected archived=true")
	}

	active, err := client.ListProjects(context.Background(), connect.NewRequest(&appv1.ListProjectsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	for _, project := range active.Msg.Projects {
		if project.Id == id {
			t.Fatal("archived project still in default list")
		}
	}

	all, err := client.ListProjects(context.Background(), connect.NewRequest(&appv1.ListProjectsRequest{IncludeArchived: true}))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, project := range all.Msg.Projects {
		if project.Id == id {
			found = true
			if !project.Archived {
				t.Fatal("include_archived list missing archived flag")
			}
		}
	}
	if !found {
		t.Fatal("archived project missing from include_archived list")
	}

	got, err := client.GetProject(context.Background(), connect.NewRequest(&appv1.GetProjectRequest{Id: id}))
	if err != nil || !got.Msg.Project.Archived {
		t.Fatalf("get archived: %#v err=%v", got, err)
	}
}

func TestTimeEntryAllocationFieldsRoundTrip(t *testing.T) {
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
	if err := seed.Dev(context.Background(), conn); err != nil {
		t.Fatal(err)
	}

	httpClient := &http.Client{Transport: handlerTransport{handler: appapi.NewHandler(appapi.Dependencies{Service: service.New(conn)})}}
	periodClient := appv1connect.NewPeriodServiceClient(httpClient, "http://shiet.test")
	scheduleClient := appv1connect.NewScheduleServiceClient(httpClient, "http://shiet.test")
	projectClient := appv1connect.NewProjectServiceClient(httpClient, "http://shiet.test")

	periods, err := periodClient.ListPeriods(context.Background(), connect.NewRequest(&appv1.ListPeriodsRequest{}))
	if err != nil || len(periods.Msg.Periods) == 0 {
		t.Fatalf("list periods: %#v err=%v", periods, err)
	}
	periodID := periods.Msg.Periods[0].Id

	project, err := projectClient.CreateProject(context.Background(), connect.NewRequest(&appv1.CreateProjectRequest{Name: "Ledger", Key: "ledger"}))
	if err != nil || project.Msg.Project == nil {
		t.Fatalf("create project: %#v err=%v", project, err)
	}
	projectID := project.Msg.Project.Id

	created, err := scheduleClient.CreateTimeEntry(context.Background(), connect.NewRequest(&appv1.CreateTimeEntryRequest{
		Input: &appv1.TimeEntryInput{
			PeriodId:       periodID,
			Day:            "2026-06-01",
			StartMinutes:   540,
			EndMinutes:     600,
			WorkType:       "paid_leave",
			ProjectId:      &projectID,
			BillableStatus: "non_billable",
			Description:    "PTO",
		},
	}))
	if err != nil {
		t.Fatal(err)
	}

	listed, err := scheduleClient.ListTimeEntries(context.Background(), connect.NewRequest(&appv1.ListTimeEntriesRequest{PeriodId: periodID}))
	if err != nil {
		t.Fatal(err)
	}
	var found *appv1.TimeEntry
	for _, entry := range listed.Msg.TimeEntries {
		if entry.Id == created.Msg.Id {
			found = entry
			break
		}
	}
	if found == nil {
		t.Fatal("created entry missing from list")
	}
	if found.WorkType != "paid_leave" || found.BillableStatus != "non_billable" {
		t.Fatalf("allocation = work_type=%q billable=%q", found.WorkType, found.BillableStatus)
	}
	if found.ProjectId == nil || *found.ProjectId != projectID {
		t.Fatalf("project_id = %v want %d", found.ProjectId, projectID)
	}

	defaults, err := scheduleClient.CreateTimeEntry(context.Background(), connect.NewRequest(&appv1.CreateTimeEntryRequest{
		Input: &appv1.TimeEntryInput{
			PeriodId:     periodID,
			Day:          "2026-06-02",
			StartMinutes: 540,
			EndMinutes:   600,
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	got, err := scheduleClient.GetTimeEntry(context.Background(), connect.NewRequest(&appv1.GetTimeEntryRequest{Id: defaults.Msg.Id, PeriodId: periodID}))
	if err != nil || got.Msg.TimeEntry == nil {
		t.Fatalf("get defaults: %#v err=%v", got, err)
	}
	if got.Msg.TimeEntry.WorkType != "worked" || got.Msg.TimeEntry.BillableStatus != "unset" || got.Msg.TimeEntry.ProjectId != nil {
		t.Fatalf("defaults = %+v", got.Msg.TimeEntry)
	}

	_, err = scheduleClient.CreateTimeEntry(context.Background(), connect.NewRequest(&appv1.CreateTimeEntryRequest{
		Input: &appv1.TimeEntryInput{
			PeriodId:     periodID,
			Day:          "2026-06-03",
			StartMinutes: 540,
			EndMinutes:   600,
			WorkType:     "overtime",
		},
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("invalid work_type code = %v", connect.CodeOf(err))
	}
}
