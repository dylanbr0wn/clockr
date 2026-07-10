package appapi

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	appv1 "github.com/dylanbr0wn/shiet/gen/shiet/app/v1"
	"github.com/dylanbr0wn/shiet/internal/service"
)

type CategoryService struct{ service *service.Service }

func (s *CategoryService) ListCategories(ctx context.Context, _ *connect.Request[appv1.ListCategoriesRequest]) (*connect.Response[appv1.ListCategoriesResponse], error) {
	items, err := s.service.ListCategories(ctx)
	if err != nil {
		return nil, mapServiceError(err)
	}
	out := make([]*appv1.Category, len(items))
	for i := range items {
		out[i] = toProtoCategory(items[i])
	}
	return connect.NewResponse(&appv1.ListCategoriesResponse{Categories: out}), nil
}

func (s *CategoryService) GetCategory(ctx context.Context, req *connect.Request[appv1.GetCategoryRequest]) (*connect.Response[appv1.GetCategoryResponse], error) {
	if err := requireID(req.Msg.Id, "id"); err != nil {
		return nil, err
	}
	item, err := s.service.GetCategory(ctx, req.Msg.Id)
	if err != nil {
		return nil, mapServiceError(err)
	}
	return connect.NewResponse(&appv1.GetCategoryResponse{Category: toProtoCategory(item)}), nil
}

func (s *CategoryService) CreateCategory(ctx context.Context, req *connect.Request[appv1.CreateCategoryRequest]) (*connect.Response[appv1.CreateCategoryResponse], error) {
	if req.Msg.Name == "" {
		return nil, invalidArgument("name is required")
	}
	if req.Msg.Color != "" {
		if err := service.ValidateCategoryColor(req.Msg.Color); err != nil {
			return nil, invalidArgument("color is invalid")
		}
	}
	item, err := s.service.CreateCategory(ctx, service.CreateCategoryInput{Name: req.Msg.Name, Description: req.Msg.Description, Key: req.Msg.Key, Color: req.Msg.Color, IsDefaultGap: req.Msg.IsDefaultGap})
	if err != nil {
		return nil, mapServiceError(err)
	}
	return connect.NewResponse(&appv1.CreateCategoryResponse{Category: toProtoCategory(item)}), nil
}

func (s *CategoryService) UpdateCategory(ctx context.Context, req *connect.Request[appv1.UpdateCategoryRequest]) (*connect.Response[appv1.UpdateCategoryResponse], error) {
	if err := requireID(req.Msg.Id, "id"); err != nil {
		return nil, err
	}
	if req.Msg.Name == "" {
		return nil, invalidArgument("name is required")
	}
	if req.Msg.Color != "" {
		if err := service.ValidateCategoryColor(req.Msg.Color); err != nil {
			return nil, invalidArgument("color is invalid")
		}
	}
	item, err := s.service.UpdateCategory(ctx, service.UpdateCategoryInput{ID: req.Msg.Id, Name: req.Msg.Name, Description: req.Msg.Description, Key: req.Msg.Key, Color: req.Msg.Color, IsDefaultGap: req.Msg.IsDefaultGap})
	if err != nil {
		return nil, mapServiceError(err)
	}
	return connect.NewResponse(&appv1.UpdateCategoryResponse{Category: toProtoCategory(item)}), nil
}

func (s *CategoryService) DeleteCategory(ctx context.Context, req *connect.Request[appv1.DeleteCategoryRequest]) (*connect.Response[appv1.DeleteCategoryResponse], error) {
	if err := requireID(req.Msg.Id, "id"); err != nil {
		return nil, err
	}
	if err := s.service.DeleteCategory(ctx, req.Msg.Id); err != nil {
		return nil, mapServiceError(err)
	}
	return connect.NewResponse(&appv1.DeleteCategoryResponse{}), nil
}

func (s *CategoryService) ListEventCategoryOverlays(ctx context.Context, req *connect.Request[appv1.ListEventCategoryOverlaysRequest]) (*connect.Response[appv1.ListEventCategoryOverlaysResponse], error) {
	if err := requireID(req.Msg.PeriodId, "period_id"); err != nil {
		return nil, err
	}
	items, err := s.service.ListEventCategoryOverlays(ctx, req.Msg.PeriodId)
	if err != nil {
		return nil, mapServiceError(err)
	}
	out := make([]*appv1.EventCategoryOverlay, len(items))
	for i, item := range items {
		out[i] = &appv1.EventCategoryOverlay{Provider: item.Provider, ExternalId: item.ExternalID, InstanceId: item.InstanceID, CategoryId: item.CategoryID}
	}
	return connect.NewResponse(&appv1.ListEventCategoryOverlaysResponse{Overlays: out}), nil
}

type CalendarService struct {
	service    *service.Service
	syncPeriod func(context.Context, int64) (service.SyncResult, error)
}

func (s *CalendarService) ListCalendars(ctx context.Context, _ *connect.Request[appv1.ListCalendarsRequest]) (*connect.Response[appv1.ListCalendarsResponse], error) {
	items, err := s.service.ListCalendars(ctx)
	if err != nil {
		return nil, mapServiceError(err)
	}
	return connect.NewResponse(&appv1.ListCalendarsResponse{Calendars: toProtoCalendars(items)}), nil
}

func (s *CalendarService) ListSelectedCalendars(ctx context.Context, _ *connect.Request[appv1.ListSelectedCalendarsRequest]) (*connect.Response[appv1.ListSelectedCalendarsResponse], error) {
	items, err := s.service.ListSelectedCalendars(ctx)
	if err != nil {
		return nil, mapServiceError(err)
	}
	return connect.NewResponse(&appv1.ListSelectedCalendarsResponse{Calendars: toProtoCalendars(items)}), nil
}

func (s *CalendarService) SetCalendarSelected(ctx context.Context, req *connect.Request[appv1.SetCalendarSelectedRequest]) (*connect.Response[appv1.SetCalendarSelectedResponse], error) {
	if err := requireID(req.Msg.CalendarId, "calendar_id"); err != nil {
		return nil, err
	}
	if err := s.service.SetCalendarSelected(ctx, req.Msg.CalendarId, req.Msg.Selected); err != nil {
		return nil, mapServiceError(err)
	}
	return connect.NewResponse(&appv1.SetCalendarSelectedResponse{}), nil
}

func (s *CalendarService) SetCalendarDefaultCategory(ctx context.Context, req *connect.Request[appv1.SetCalendarDefaultCategoryRequest]) (*connect.Response[appv1.SetCalendarDefaultCategoryResponse], error) {
	if err := requireID(req.Msg.CalendarId, "calendar_id"); err != nil {
		return nil, err
	}
	if err := s.service.SetCalendarDefaultCategory(ctx, req.Msg.CalendarId, req.Msg.CategoryId); err != nil {
		return nil, mapServiceError(err)
	}
	return connect.NewResponse(&appv1.SetCalendarDefaultCategoryResponse{}), nil
}

func (s *CalendarService) SyncPeriod(ctx context.Context, req *connect.Request[appv1.SyncPeriodRequest]) (*connect.Response[appv1.SyncPeriodResponse], error) {
	if err := requireID(req.Msg.PeriodId, "period_id"); err != nil {
		return nil, err
	}
	if s.syncPeriod == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("sync is unavailable"))
	}
	result, err := s.syncPeriod(ctx, req.Msg.PeriodId)
	if err != nil {
		return nil, mapServiceError(err)
	}
	return connect.NewResponse(&appv1.SyncPeriodResponse{Added: int32(result.Added), Updated: int32(result.Updated), Unchanged: int32(result.Unchanged), Removed: int32(result.Removed), Flagged: int32(result.Flagged)}), nil
}

func toProtoCategory(item service.Category) *appv1.Category {
	return &appv1.Category{Id: item.ID, Name: item.Name, Description: item.Description, Key: item.Key, Color: item.Color, IsDefaultGap: item.IsDefaultGap}
}

func toProtoCalendars(items []service.Calendar) []*appv1.Calendar {
	out := make([]*appv1.Calendar, len(items))
	for i, item := range items {
		out[i] = &appv1.Calendar{Id: item.ID, Provider: item.Provider, ExternalId: item.ExternalID, Name: item.Name, IsPrimary: item.IsPrimary, Selected: item.Selected, DefaultCategoryId: item.DefaultCategoryID}
	}
	return out
}
