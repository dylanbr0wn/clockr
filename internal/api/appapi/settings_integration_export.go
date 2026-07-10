package appapi

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	appv1 "github.com/dylanbr0wn/shiet/gen/shiet/app/v1"
	"github.com/dylanbr0wn/shiet/internal/integration/connection"
	"github.com/dylanbr0wn/shiet/internal/service"
)

type SettingsService struct{ service *service.Service }

func (s *SettingsService) GetSetting(ctx context.Context, req *connect.Request[appv1.GetSettingRequest]) (*connect.Response[appv1.GetSettingResponse], error) {
	if req.Msg.Key == "" {
		return nil, invalidArgument("key is required")
	}
	value, err := s.service.GetSetting(ctx, req.Msg.Key)
	if err != nil {
		return nil, mapServiceError(err)
	}
	return connect.NewResponse(&appv1.GetSettingResponse{Value: value}), nil
}

func (s *SettingsService) SetSetting(ctx context.Context, req *connect.Request[appv1.SetSettingRequest]) (*connect.Response[appv1.SetSettingResponse], error) {
	if req.Msg.Key == "" {
		return nil, invalidArgument("key is required")
	}
	if err := s.service.SetSetting(ctx, req.Msg.Key, req.Msg.Value); err != nil {
		return nil, mapServiceError(err)
	}
	return connect.NewResponse(&appv1.SetSettingResponse{}), nil
}

type IntegrationService struct {
	service              *service.Service
	listConnections      func(context.Context) ([]connection.Connection, error)
	refreshGitHubRepos   func(context.Context, string) error
	refreshSlackChannels func(context.Context, string) error
}

func (s *IntegrationService) ListIntegrationConnections(ctx context.Context, _ *connect.Request[appv1.ListIntegrationConnectionsRequest]) (*connect.Response[appv1.ListIntegrationConnectionsResponse], error) {
	if s.listConnections == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("integration registry is unavailable"))
	}
	items, err := s.listConnections(ctx)
	if err != nil {
		return nil, mapServiceError(err)
	}
	out := make([]*appv1.IntegrationConnection, len(items))
	for i, item := range items {
		out[i] = &appv1.IntegrationConnection{Id: item.ID, Provider: item.Provider, AccountLabel: item.AccountLabel, AccountId: item.AccountID, Scopes: append([]string(nil), item.Scopes...), Status: item.Status, ConnectedAt: item.ConnectedAt, UpdatedAt: item.UpdatedAt}
	}
	return connect.NewResponse(&appv1.ListIntegrationConnectionsResponse{Connections: out}), nil
}

func (s *IntegrationService) ListGitHubRepos(ctx context.Context, _ *connect.Request[appv1.ListGitHubReposRequest]) (*connect.Response[appv1.ListGitHubReposResponse], error) {
	items, err := s.service.ListGitHubRepos(ctx)
	if err != nil {
		return nil, mapServiceError(err)
	}
	out := make([]*appv1.GitHubRepo, len(items))
	for i, item := range items {
		out[i] = &appv1.GitHubRepo{Id: item.ID, AccountId: item.AccountID, ExternalId: item.ExternalID, Name: item.Name, FullName: item.FullName, Private: item.Private, Selected: item.Selected}
	}
	return connect.NewResponse(&appv1.ListGitHubReposResponse{Repos: out}), nil
}

func (s *IntegrationService) SetGitHubRepoSelected(ctx context.Context, req *connect.Request[appv1.SetGitHubRepoSelectedRequest]) (*connect.Response[appv1.SetGitHubRepoSelectedResponse], error) {
	if err := requireID(req.Msg.RepoId, "repo_id"); err != nil {
		return nil, err
	}
	if err := s.service.SetGitHubRepoSelected(ctx, req.Msg.RepoId, req.Msg.Selected); err != nil {
		return nil, mapServiceError(err)
	}
	return connect.NewResponse(&appv1.SetGitHubRepoSelectedResponse{}), nil
}

func (s *IntegrationService) RefreshGitHubRepos(ctx context.Context, req *connect.Request[appv1.RefreshGitHubReposRequest]) (*connect.Response[appv1.RefreshGitHubReposResponse], error) {
	if req.Msg.AccountId == "" {
		return nil, invalidArgument("account_id is required")
	}
	if s.refreshGitHubRepos == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("GitHub refresh is unavailable"))
	}
	if err := s.refreshGitHubRepos(ctx, req.Msg.AccountId); err != nil {
		return nil, mapServiceError(err)
	}
	return connect.NewResponse(&appv1.RefreshGitHubReposResponse{}), nil
}

func (s *IntegrationService) ListSlackChannels(ctx context.Context, _ *connect.Request[appv1.ListSlackChannelsRequest]) (*connect.Response[appv1.ListSlackChannelsResponse], error) {
	items, err := s.service.ListSlackChannels(ctx)
	if err != nil {
		return nil, mapServiceError(err)
	}
	out := make([]*appv1.SlackChannel, len(items))
	for i, item := range items {
		out[i] = &appv1.SlackChannel{Id: item.ID, AccountId: item.AccountID, ExternalId: item.ExternalID, Name: item.Name, Private: item.Private, Selected: item.Selected}
	}
	return connect.NewResponse(&appv1.ListSlackChannelsResponse{Channels: out}), nil
}

func (s *IntegrationService) SetSlackChannelSelected(ctx context.Context, req *connect.Request[appv1.SetSlackChannelSelectedRequest]) (*connect.Response[appv1.SetSlackChannelSelectedResponse], error) {
	if err := requireID(req.Msg.ChannelId, "channel_id"); err != nil {
		return nil, err
	}
	if err := s.service.SetSlackChannelSelected(ctx, req.Msg.ChannelId, req.Msg.Selected); err != nil {
		return nil, mapServiceError(err)
	}
	return connect.NewResponse(&appv1.SetSlackChannelSelectedResponse{}), nil
}

func (s *IntegrationService) RefreshSlackChannels(ctx context.Context, req *connect.Request[appv1.RefreshSlackChannelsRequest]) (*connect.Response[appv1.RefreshSlackChannelsResponse], error) {
	if req.Msg.AccountId == "" {
		return nil, invalidArgument("account_id is required")
	}
	if s.refreshSlackChannels == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("Slack refresh is unavailable"))
	}
	if err := s.refreshSlackChannels(ctx, req.Msg.AccountId); err != nil {
		return nil, mapServiceError(err)
	}
	return connect.NewResponse(&appv1.RefreshSlackChannelsResponse{}), nil
}

type ExportService struct{ service *service.Service }

func (s *ExportService) RenderPeriodExport(ctx context.Context, req *connect.Request[appv1.RenderPeriodExportRequest]) (*connect.Response[appv1.RenderPeriodExportResponse], error) {
	if err := requireID(req.Msg.PeriodId, "period_id"); err != nil {
		return nil, err
	}
	render, err := s.service.RenderPeriodExport(ctx, req.Msg.PeriodId, req.Msg.TemplateKey)
	if err != nil {
		return nil, mapServiceError(err)
	}
	return connect.NewResponse(&appv1.RenderPeriodExportResponse{Filename: render.Filename, Content: render.Content, Format: render.Format}), nil
}

func (s *ExportService) BuildPeriodExport(ctx context.Context, req *connect.Request[appv1.BuildPeriodExportRequest]) (*connect.Response[appv1.BuildPeriodExportResponse], error) {
	if err := requireID(req.Msg.PeriodId, "period_id"); err != nil {
		return nil, err
	}
	model, err := s.service.BuildPeriodExport(ctx, req.Msg.PeriodId)
	if err != nil {
		return nil, mapServiceError(err)
	}
	entries := make([]*appv1.ExportEntry, len(model.Entries))
	for i, item := range model.Entries {
		entries[i] = &appv1.ExportEntry{Source: item.Source, SourceId: item.SourceID, Day: item.Day, StartMinutes: int32(item.StartMinutes), EndMinutes: int32(item.EndMinutes), Minutes: int32(item.Minutes), Title: item.Title, Category: toProtoExportCategory(item.Category)}
	}
	daily := make([]*appv1.ExportDayTotals, len(model.DailyTotals))
	for i, item := range model.DailyTotals {
		daily[i] = &appv1.ExportDayTotals{Date: item.Date, Categories: toProtoExportCategoryMinutes(item.Categories), ActualMinutes: int32(item.ActualMinutes), TargetMinutes: int32(item.TargetMinutes)}
	}
	return connect.NewResponse(&appv1.BuildPeriodExportResponse{PeriodId: model.PeriodID, PeriodLabel: model.PeriodLabel, StartDate: model.StartDate, EndDate: model.EndDate, TargetHoursPerDay: model.TargetHoursPerDay, TargetMinutes: int32(model.TargetMinutes), ActualMinutes: int32(model.ActualMinutes), Days: append([]string(nil), model.Days...), Entries: entries, DailyTotals: daily, PeriodTotals: toProtoExportCategoryMinutes(model.PeriodTotals)}), nil
}

func (s *ExportService) ListExportTemplates(ctx context.Context, _ *connect.Request[appv1.ListExportTemplatesRequest]) (*connect.Response[appv1.ListExportTemplatesResponse], error) {
	items, err := s.service.ListExportTemplates(ctx)
	if err != nil {
		return nil, mapServiceError(err)
	}
	out := make([]*appv1.ExportTemplate, len(items))
	for i := range items {
		out[i] = toProtoExportTemplate(items[i])
	}
	return connect.NewResponse(&appv1.ListExportTemplatesResponse{Templates: out}), nil
}

func (s *ExportService) GetExportTemplate(ctx context.Context, req *connect.Request[appv1.GetExportTemplateRequest]) (*connect.Response[appv1.GetExportTemplateResponse], error) {
	if req.Msg.Key == "" {
		return nil, invalidArgument("key is required")
	}
	item, err := s.service.GetExportTemplate(ctx, req.Msg.Key)
	if err != nil {
		return nil, mapServiceError(err)
	}
	return connect.NewResponse(&appv1.GetExportTemplateResponse{Template: toProtoExportTemplate(item)}), nil
}

func (s *ExportService) CreateExportTemplate(ctx context.Context, req *connect.Request[appv1.CreateExportTemplateRequest]) (*connect.Response[appv1.CreateExportTemplateResponse], error) {
	if req.Msg.Name == "" {
		return nil, invalidArgument("name is required")
	}
	if req.Msg.Format == "" {
		return nil, invalidArgument("format is required")
	}
	item, err := s.service.CreateExportTemplate(ctx, service.CreateExportTemplateInput{Key: req.Msg.Key, Name: req.Msg.Name, Description: req.Msg.Description, Format: req.Msg.Format, Body: req.Msg.Body})
	if err != nil {
		return nil, mapServiceError(err)
	}
	return connect.NewResponse(&appv1.CreateExportTemplateResponse{Template: toProtoExportTemplate(item)}), nil
}

func (s *ExportService) UpdateExportTemplate(ctx context.Context, req *connect.Request[appv1.UpdateExportTemplateRequest]) (*connect.Response[appv1.UpdateExportTemplateResponse], error) {
	if err := requireID(req.Msg.Id, "id"); err != nil {
		return nil, err
	}
	if req.Msg.Name == "" {
		return nil, invalidArgument("name is required")
	}
	if req.Msg.Format == "" {
		return nil, invalidArgument("format is required")
	}
	item, err := s.service.UpdateExportTemplate(ctx, service.UpdateExportTemplateInput{ID: req.Msg.Id, Name: req.Msg.Name, Description: req.Msg.Description, Format: req.Msg.Format, Body: req.Msg.Body})
	if err != nil {
		return nil, mapServiceError(err)
	}
	return connect.NewResponse(&appv1.UpdateExportTemplateResponse{Template: toProtoExportTemplate(item)}), nil
}

func (s *ExportService) DeleteExportTemplate(ctx context.Context, req *connect.Request[appv1.DeleteExportTemplateRequest]) (*connect.Response[appv1.DeleteExportTemplateResponse], error) {
	if err := requireID(req.Msg.Id, "id"); err != nil {
		return nil, err
	}
	if err := s.service.DeleteExportTemplate(ctx, req.Msg.Id); err != nil {
		return nil, mapServiceError(err)
	}
	return connect.NewResponse(&appv1.DeleteExportTemplateResponse{}), nil
}

func (s *ExportService) DuplicateExportTemplate(ctx context.Context, req *connect.Request[appv1.DuplicateExportTemplateRequest]) (*connect.Response[appv1.DuplicateExportTemplateResponse], error) {
	item, err := s.service.DuplicateExportTemplate(ctx, req.Msg.Key)
	if err != nil {
		return nil, mapServiceError(err)
	}
	return connect.NewResponse(&appv1.DuplicateExportTemplateResponse{Template: toProtoExportTemplate(item)}), nil
}

func (s *ExportService) PreviewExport(ctx context.Context, req *connect.Request[appv1.PreviewExportRequest]) (*connect.Response[appv1.PreviewExportResponse], error) {
	if err := requireID(req.Msg.PeriodId, "period_id"); err != nil {
		return nil, err
	}
	render, err := s.service.PreviewExport(ctx, service.PreviewExportInput{PeriodID: req.Msg.PeriodId, TemplateKey: req.Msg.TemplateKey, Format: req.Msg.Format, Body: req.Msg.Body})
	if err != nil {
		return nil, mapServiceError(err)
	}
	return connect.NewResponse(&appv1.PreviewExportResponse{Filename: render.Filename, Content: render.Content, Format: render.Format}), nil
}

func (s *ExportService) ListExportFieldCatalog(_ context.Context, req *connect.Request[appv1.ListExportFieldCatalogRequest]) (*connect.Response[appv1.ListExportFieldCatalogResponse], error) {
	items, err := service.ListExportFieldCatalog(req.Msg.Grain, req.Msg.Layout)
	if err != nil {
		return nil, mapServiceError(err)
	}
	out := make([]*appv1.ExportFieldInfo, len(items))
	for i, item := range items {
		out[i] = &appv1.ExportFieldInfo{Field: item.Field, Label: item.Label, Description: item.Description}
	}
	return connect.NewResponse(&appv1.ListExportFieldCatalogResponse{Fields: out}), nil
}

func toProtoExportTemplate(item service.ExportTemplate) *appv1.ExportTemplate {
	return &appv1.ExportTemplate{Id: item.ID, Key: item.Key, Name: item.Name, Description: item.Description, Format: item.Format, Builtin: item.Builtin, Body: item.Body}
}

func toProtoExportCategory(item service.ExportCategory) *appv1.ExportCategory {
	return &appv1.ExportCategory{Id: item.ID, Name: item.Name, Key: item.Key, Color: item.Color}
}

func toProtoExportCategoryMinutes(items []service.ExportCategoryMinutes) []*appv1.ExportCategoryMinutes {
	out := make([]*appv1.ExportCategoryMinutes, len(items))
	for i, item := range items {
		out[i] = &appv1.ExportCategoryMinutes{Category: toProtoExportCategory(item.Category), Minutes: int32(item.Minutes)}
	}
	return out
}
