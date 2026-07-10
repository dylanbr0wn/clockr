package appapi

import (
	"context"
	"errors"
	"net/http"

	"connectrpc.com/connect"
	"github.com/dylanbr0wn/shiet/gen/shiet/app/v1/appv1connect"
	"github.com/dylanbr0wn/shiet/internal/integration/connection"
	"github.com/dylanbr0wn/shiet/internal/service"
)

type Dependencies struct {
	Service              *service.Service
	SyncPeriod           func(context.Context, int64) (service.SyncResult, error)
	ListConnections      func(context.Context) ([]connection.Connection, error)
	RefreshGitHubRepos   func(context.Context, string) error
	RefreshSlackChannels func(context.Context, string) error
}

func NewHandler(deps Dependencies) http.Handler {
	mux := http.NewServeMux()
	mount := func(path string, handler http.Handler) { mux.Handle(path, handler) }

	path, handler := appv1connect.NewPeriodServiceHandler(NewPeriodService(deps.Service))
	mount(path, handler)
	path, handler = appv1connect.NewCategoryServiceHandler(&CategoryService{service: deps.Service})
	mount(path, handler)
	path, handler = appv1connect.NewCalendarServiceHandler(&CalendarService{service: deps.Service, syncPeriod: deps.SyncPeriod})
	mount(path, handler)
	path, handler = appv1connect.NewScheduleServiceHandler(&ScheduleService{service: deps.Service})
	mount(path, handler)
	path, handler = appv1connect.NewSettingsServiceHandler(&SettingsService{service: deps.Service})
	mount(path, handler)
	path, handler = appv1connect.NewIntegrationServiceHandler(&IntegrationService{service: deps.Service, listConnections: deps.ListConnections, refreshGitHubRepos: deps.RefreshGitHubRepos, refreshSlackChannels: deps.RefreshSlackChannels})
	mount(path, handler)
	path, handler = appv1connect.NewExportServiceHandler(&ExportService{service: deps.Service})
	mount(path, handler)
	return mux
}

func requireID(id int64, name string) error {
	if id <= 0 {
		return invalidArgument(name + " is required")
	}
	return nil
}

func invalidArgument(message string) error {
	return connect.NewError(connect.CodeInvalidArgument, errors.New(message))
}
