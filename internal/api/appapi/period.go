// Package appapi exposes portable application operations through Connect.
package appapi

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	appv1 "github.com/dylanbr0wn/shiet/gen/shiet/app/v1"
	"github.com/dylanbr0wn/shiet/internal/service"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type PeriodService struct {
	service *service.Service
}

func NewPeriodService(svc *service.Service) *PeriodService {
	return &PeriodService{service: svc}
}

func (s *PeriodService) ListPeriods(ctx context.Context, _ *connect.Request[appv1.ListPeriodsRequest]) (*connect.Response[appv1.ListPeriodsResponse], error) {
	periods, err := s.service.ListPeriods(ctx)
	if err != nil {
		return nil, mapServiceError(err)
	}
	out := &appv1.ListPeriodsResponse{Periods: make([]*appv1.Period, len(periods))}
	for i := range periods {
		out.Periods[i] = toProtoPeriod(periods[i])
	}
	return connect.NewResponse(out), nil
}

func (s *PeriodService) GetPeriod(ctx context.Context, req *connect.Request[appv1.GetPeriodRequest]) (*connect.Response[appv1.GetPeriodResponse], error) {
	if err := requireID(req.Msg.Id, "id"); err != nil {
		return nil, err
	}
	period, err := s.service.GetPeriod(ctx, req.Msg.Id)
	if err != nil {
		return nil, mapServiceError(err)
	}
	return connect.NewResponse(&appv1.GetPeriodResponse{Period: toProtoPeriod(period)}), nil
}

func (s *PeriodService) GetPeriodByRange(ctx context.Context, req *connect.Request[appv1.GetPeriodByRangeRequest]) (*connect.Response[appv1.GetPeriodByRangeResponse], error) {
	if _, err := time.Parse("2006-01-02", req.Msg.StartDate); err != nil {
		return nil, invalidArgument("start_date must be a YYYY-MM-DD date")
	}
	if _, err := time.Parse("2006-01-02", req.Msg.EndDate); err != nil {
		return nil, invalidArgument("end_date must be a YYYY-MM-DD date")
	}
	period, err := s.service.GetPeriodByRange(ctx, req.Msg.StartDate, req.Msg.EndDate)
	if err != nil {
		return nil, mapServiceError(err)
	}
	return connect.NewResponse(&appv1.GetPeriodByRangeResponse{Period: toProtoPeriod(period)}), nil
}

func (s *PeriodService) EnsureCurrentPeriod(ctx context.Context, req *connect.Request[appv1.EnsureCurrentPeriodRequest]) (*connect.Response[appv1.EnsureCurrentPeriodResponse], error) {
	if _, err := time.Parse("2006-01-02", req.Msg.Today); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("today must be a YYYY-MM-DD date"))
	}
	ianaTZ := req.Msg.IanaTz
	if ianaTZ == "" {
		ianaTZ = "UTC"
	}
	if _, err := time.LoadLocation(ianaTZ); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("iana_tz must be a valid IANA timezone"))
	}
	period, err := s.service.EnsureCurrentPeriod(ctx, req.Msg.Today, ianaTZ)
	if err != nil {
		return nil, mapServiceError(err)
	}
	return connect.NewResponse(&appv1.EnsureCurrentPeriodResponse{Period: toProtoPeriod(period)}), nil
}

func toProtoPeriod(period service.Period) *appv1.Period {
	out := &appv1.Period{
		Id:                period.ID,
		StartDate:         period.StartDate,
		EndDate:           period.EndDate,
		Cadence:           period.Cadence,
		AnchorDate:        period.AnchorDate,
		TargetHoursPerDay: period.TargetHoursPerDay,
	}
	if period.LastSyncedAt != nil {
		out.LastSyncedAt = timestamppb.New(*period.LastSyncedAt)
	}
	return out
}

func mapServiceError(err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return connect.NewError(connect.CodeCanceled, errors.New("request canceled"))
	case errors.Is(err, context.DeadlineExceeded):
		return connect.NewError(connect.CodeDeadlineExceeded, errors.New("request deadline exceeded"))
	case errors.Is(err, service.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("resource not found"))
	case errors.Is(err, service.ErrInvalidInput):
		return connect.NewError(connect.CodeInvalidArgument, errors.New("request is invalid"))
	case errors.Is(err, service.ErrFailedPrecondition):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("operation precondition is not satisfied"))
	case errors.Is(err, service.ErrCategoryInUse):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("category is in use"))
	case errors.Is(err, service.ErrExportTemplateBuiltin):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("builtin export templates cannot be modified"))
	case errors.Is(err, service.ErrCalendarSyncNotConfigured), errors.Is(err, service.ErrNoConnectedAccounts):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("calendar sync is not configured"))
	case errors.Is(err, service.ErrNeedsReauth):
		return connect.NewError(connect.CodeUnauthenticated, errors.New("calendar connection needs reauthentication"))
	default:
		return connect.NewError(connect.CodeInternal, errors.New("internal service error"))
	}
}
