package appapi

import (
	"context"

	"connectrpc.com/connect"
	appv1 "github.com/dylanbr0wn/shiet/gen/shiet/app/v1"
	"github.com/dylanbr0wn/shiet/internal/service"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ScheduleService struct{ service *service.Service }

func (s *ScheduleService) ListEvents(ctx context.Context, req *connect.Request[appv1.ListEventsRequest]) (*connect.Response[appv1.ListEventsResponse], error) {
	if err := requireID(req.Msg.PeriodId, "period_id"); err != nil {
		return nil, err
	}
	items, err := s.service.ListEvents(ctx, req.Msg.PeriodId)
	if err != nil {
		return nil, mapServiceError(err)
	}
	out := make([]*appv1.Event, len(items))
	for i := range items {
		out[i] = toProtoEvent(items[i])
	}
	return connect.NewResponse(&appv1.ListEventsResponse{Events: out}), nil
}

func (s *ScheduleService) GetEvent(ctx context.Context, req *connect.Request[appv1.GetEventRequest]) (*connect.Response[appv1.GetEventResponse], error) {
	if err := requireID(req.Msg.Id, "id"); err != nil {
		return nil, err
	}
	item, err := s.service.GetEvent(ctx, req.Msg.Id)
	if err != nil {
		return nil, mapServiceError(err)
	}
	return connect.NewResponse(&appv1.GetEventResponse{Event: toProtoEvent(item)}), nil
}

func (s *ScheduleService) ListGapFills(ctx context.Context, req *connect.Request[appv1.ListGapFillsRequest]) (*connect.Response[appv1.ListGapFillsResponse], error) {
	if err := requireID(req.Msg.PeriodId, "period_id"); err != nil {
		return nil, err
	}
	items, err := s.service.ListGapFills(ctx, req.Msg.PeriodId)
	if err != nil {
		return nil, mapServiceError(err)
	}
	out := make([]*appv1.GapFill, len(items))
	for i := range items {
		out[i] = toProtoGapFill(items[i])
	}
	return connect.NewResponse(&appv1.ListGapFillsResponse{GapFills: out}), nil
}

func (s *ScheduleService) CreateManualEvent(ctx context.Context, req *connect.Request[appv1.CreateManualEventRequest]) (*connect.Response[appv1.CreateManualEventResponse], error) {
	input, err := manualEventInput(req.Msg.Input)
	if err != nil {
		return nil, err
	}
	item, serviceErr := s.service.CreateManualEvent(ctx, input)
	if serviceErr != nil {
		return nil, mapServiceError(serviceErr)
	}
	return connect.NewResponse(&appv1.CreateManualEventResponse{PeriodId: item.PeriodID, Id: item.ID}), nil
}

func (s *ScheduleService) CreateGapFill(ctx context.Context, req *connect.Request[appv1.CreateGapFillRequest]) (*connect.Response[appv1.CreateGapFillResponse], error) {
	input, err := manualEventInput(req.Msg.Input)
	if err != nil {
		return nil, err
	}
	item, serviceErr := s.service.CreateGapFill(ctx, input)
	if serviceErr != nil {
		return nil, mapServiceError(serviceErr)
	}
	return connect.NewResponse(&appv1.CreateGapFillResponse{PeriodId: item.PeriodID, Id: item.ID}), nil
}

func (s *ScheduleService) UpdateManualEvent(ctx context.Context, req *connect.Request[appv1.UpdateManualEventRequest]) (*connect.Response[appv1.UpdateManualEventResponse], error) {
	if err := requireID(req.Msg.Id, "id"); err != nil {
		return nil, err
	}
	input, err := manualEventInput(req.Msg.Input)
	if err != nil {
		return nil, err
	}
	item, serviceErr := s.service.UpdateManualEvent(ctx, service.ManualEventUpdateInput{ID: req.Msg.Id, ManualEventInput: input})
	if serviceErr != nil {
		return nil, mapServiceError(serviceErr)
	}
	return connect.NewResponse(&appv1.UpdateManualEventResponse{PeriodId: item.PeriodID, Id: item.ID}), nil
}

func (s *ScheduleService) DeleteManualEvent(ctx context.Context, req *connect.Request[appv1.DeleteManualEventRequest]) (*connect.Response[appv1.DeleteManualEventResponse], error) {
	if err := requireID(req.Msg.Id, "id"); err != nil {
		return nil, err
	}
	if err := requireID(req.Msg.PeriodId, "period_id"); err != nil {
		return nil, err
	}
	if err := s.service.DeleteManualEvent(ctx, service.ManualEventDeleteInput{ID: req.Msg.Id, PeriodID: req.Msg.PeriodId}); err != nil {
		return nil, mapServiceError(err)
	}
	return connect.NewResponse(&appv1.DeleteManualEventResponse{PeriodId: req.Msg.PeriodId, Id: req.Msg.Id}), nil
}

func (s *ScheduleService) ListReviewDecisions(ctx context.Context, req *connect.Request[appv1.ListReviewDecisionsRequest]) (*connect.Response[appv1.ListReviewDecisionsResponse], error) {
	if err := requireID(req.Msg.PeriodId, "period_id"); err != nil {
		return nil, err
	}
	items, err := s.service.ListReviewDecisions(ctx, req.Msg.PeriodId)
	if err != nil {
		return nil, mapServiceError(err)
	}
	out := make([]*appv1.ReviewDecision, len(items))
	for i, item := range items {
		actions := make([]*appv1.ReviewDecisionAction, len(item.Actions))
		for j, action := range item.Actions {
			actions[j] = &appv1.ReviewDecisionAction{Key: action.Key, Label: action.Label, Role: toProtoReviewRole(action.Role), Variant: toProtoReviewVariant(action.Variant)}
		}
		out[i] = &appv1.ReviewDecision{Id: item.ID, Kind: item.Kind, EventId: item.EventID, Tag: item.Tag, Title: item.Title, Description: item.Description, Actions: actions}
	}
	return connect.NewResponse(&appv1.ListReviewDecisionsResponse{Decisions: out}), nil
}

func toProtoReviewRole(role string) appv1.ReviewActionRole {
	if role == "primary" {
		return appv1.ReviewActionRole_REVIEW_ACTION_ROLE_PRIMARY
	}
	if role == "secondary" {
		return appv1.ReviewActionRole_REVIEW_ACTION_ROLE_SECONDARY
	}
	return appv1.ReviewActionRole_REVIEW_ACTION_ROLE_UNSPECIFIED
}

func toProtoReviewVariant(variant string) appv1.ReviewActionVariant {
	switch variant {
	case "default":
		return appv1.ReviewActionVariant_REVIEW_ACTION_VARIANT_DEFAULT
	case "outline":
		return appv1.ReviewActionVariant_REVIEW_ACTION_VARIANT_OUTLINE
	case "destructive":
		return appv1.ReviewActionVariant_REVIEW_ACTION_VARIANT_DESTRUCTIVE
	default:
		return appv1.ReviewActionVariant_REVIEW_ACTION_VARIANT_UNSPECIFIED
	}
}

func (s *ScheduleService) ResolveReviewDecision(ctx context.Context, req *connect.Request[appv1.ResolveReviewDecisionRequest]) (*connect.Response[appv1.ResolveReviewDecisionResponse], error) {
	if err := requireID(req.Msg.DecisionId, "decision_id"); err != nil {
		return nil, err
	}
	result, err := s.service.ResolveReviewDecision(ctx, service.ResolveReviewDecisionInput{DecisionID: req.Msg.DecisionId, Action: req.Msg.Action})
	if err != nil {
		return nil, mapServiceError(err)
	}
	return connect.NewResponse(&appv1.ResolveReviewDecisionResponse{PeriodId: result.PeriodID}), nil
}

func (s *ScheduleService) ExcludeEvent(ctx context.Context, req *connect.Request[appv1.ExcludeEventRequest]) (*connect.Response[appv1.ExcludeEventResponse], error) {
	if err := requireID(req.Msg.EventId, "event_id"); err != nil {
		return nil, err
	}
	if err := requireID(req.Msg.PeriodId, "period_id"); err != nil {
		return nil, err
	}
	result, err := s.service.ExcludeEvent(ctx, service.ExcludeEventInput{EventID: req.Msg.EventId, PeriodID: req.Msg.PeriodId})
	if err != nil {
		return nil, mapServiceError(err)
	}
	return connect.NewResponse(&appv1.ExcludeEventResponse{PeriodId: result.PeriodID, EventId: result.EventID}), nil
}

func (s *ScheduleService) ListTzSegments(ctx context.Context, req *connect.Request[appv1.ListTzSegmentsRequest]) (*connect.Response[appv1.ListTzSegmentsResponse], error) {
	if err := requireID(req.Msg.PeriodId, "period_id"); err != nil {
		return nil, err
	}
	items, err := s.service.ListTzSegments(ctx, req.Msg.PeriodId)
	if err != nil {
		return nil, mapServiceError(err)
	}
	out := make([]*appv1.TzSegment, len(items))
	for i, item := range items {
		out[i] = &appv1.TzSegment{Id: item.ID, PeriodId: item.PeriodID, EffectiveFromDate: item.EffectiveFromDate, IanaTz: item.IanaTz}
	}
	return connect.NewResponse(&appv1.ListTzSegmentsResponse{Segments: out}), nil
}

func (s *ScheduleService) ComputeGaps(ctx context.Context, req *connect.Request[appv1.ComputeGapsRequest]) (*connect.Response[appv1.ComputeGapsResponse], error) {
	if err := requireID(req.Msg.PeriodId, "period_id"); err != nil {
		return nil, err
	}
	items, err := s.service.ComputeGaps(ctx, req.Msg.PeriodId)
	if err != nil {
		return nil, mapServiceError(err)
	}
	out := make([]*appv1.DayTimeline, len(items))
	for i, item := range items {
		out[i] = toProtoDayTimeline(item)
	}
	return connect.NewResponse(&appv1.ComputeGapsResponse{Days: out}), nil
}

func (s *ScheduleService) SuggestGapFill(ctx context.Context, req *connect.Request[appv1.SuggestGapFillRequest]) (*connect.Response[appv1.SuggestGapFillResponse], error) {
	if req.Msg.Start == nil || req.Msg.End == nil {
		return nil, invalidArgument("start and end are required")
	}
	if err := req.Msg.Start.CheckValid(); err != nil {
		return nil, invalidArgument("start must be a valid timestamp")
	}
	if err := req.Msg.End.CheckValid(); err != nil {
		return nil, invalidArgument("end must be a valid timestamp")
	}
	result, err := s.service.SuggestGapFill(ctx, service.TimeWindow{Start: req.Msg.Start.AsTime(), End: req.Msg.End.AsTime()})
	if err != nil {
		return nil, mapServiceError(err)
	}
	return connect.NewResponse(&appv1.SuggestGapFillResponse{Category: result.Category, Description: result.Description, EvidenceCount: int32(result.EvidenceCount)}), nil
}

func manualEventInput(input *appv1.ManualEventInput) (service.ManualEventInput, error) {
	if input == nil {
		return service.ManualEventInput{}, invalidArgument("input is required")
	}
	if err := requireID(input.PeriodId, "period_id"); err != nil {
		return service.ManualEventInput{}, err
	}
	if input.Day == "" {
		return service.ManualEventInput{}, invalidArgument("day is required")
	}
	if input.StartMinutes < 0 || input.EndMinutes > 24*60 || input.StartMinutes >= input.EndMinutes {
		return service.ManualEventInput{}, invalidArgument("manual event minute range is invalid")
	}
	return service.ManualEventInput{PeriodID: input.PeriodId, Day: input.Day, StartMinutes: int(input.StartMinutes), EndMinutes: int(input.EndMinutes), CategoryID: input.CategoryId, Note: input.Note, Description: input.Description}, nil
}

func toProtoEvent(item service.Event) *appv1.Event {
	attendees := make([]*appv1.Attendee, len(item.Attendees))
	for i, attendee := range item.Attendees {
		attendees[i] = &appv1.Attendee{Email: attendee.Email, DisplayName: attendee.DisplayName, ResponseStatus: attendee.ResponseStatus, Organizer: attendee.Organizer, Self: attendee.Self}
	}
	out := &appv1.Event{Id: item.ID, PeriodId: item.PeriodID, CalendarId: item.CalendarID, Provider: item.Provider, ExternalId: item.ExternalID, InstanceId: item.InstanceID, RecurringEventId: item.RecurringEventID, IcalUid: item.ICalUID, Title: item.Title, Description: item.Description, Location: item.Location, Organizer: item.Organizer, Attendees: attendees, Status: item.Status, AllDay: item.AllDay, StartDate: item.StartDate, EndDate: item.EndDate, OriginalTz: item.OriginalTz, Active: item.Active}
	if item.Start != nil {
		out.Start = timestamppb.New(*item.Start)
	}
	if item.End != nil {
		out.End = timestamppb.New(*item.End)
	}
	return out
}

func toProtoGapFill(item service.GapFill) *appv1.GapFill {
	return &appv1.GapFill{Id: item.ID, PeriodId: item.PeriodID, Day: item.Day, Start: item.Start, End: item.End, CategoryId: item.CategoryID, Note: item.Note, Description: item.Description, Source: item.Source}
}

func toProtoDayTimeline(item service.DayTimeline) *appv1.DayTimeline {
	return &appv1.DayTimeline{Date: item.Date, Tz: item.Tz, WindowStart: timestamppb.New(item.WindowStart), WindowEnd: timestamppb.New(item.WindowEnd), Events: toProtoIntervals(item.Events), Filled: toProtoIntervals(item.Filled), Gaps: toProtoIntervals(item.Gaps), CoveredHours: item.CoveredHours, GapHours: item.GapHours}
}

func toProtoIntervals(items []service.Interval) []*appv1.Interval {
	out := make([]*appv1.Interval, len(items))
	for i, item := range items {
		out[i] = &appv1.Interval{Start: timestamppb.New(item.Start), End: timestamppb.New(item.End)}
	}
	return out
}
