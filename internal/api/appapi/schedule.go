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

func (s *ScheduleService) ListTimeEntries(ctx context.Context, req *connect.Request[appv1.ListTimeEntriesRequest]) (*connect.Response[appv1.ListTimeEntriesResponse], error) {
	if err := requireID(req.Msg.PeriodId, "period_id"); err != nil {
		return nil, err
	}
	items, err := s.service.ListTimeEntries(ctx, req.Msg.PeriodId)
	if err != nil {
		return nil, mapServiceError(err)
	}
	out := make([]*appv1.TimeEntry, len(items))
	for i := range items {
		out[i] = toProtoTimeEntry(items[i])
	}
	return connect.NewResponse(&appv1.ListTimeEntriesResponse{TimeEntries: out}), nil
}

func (s *ScheduleService) GetTimeEntry(ctx context.Context, req *connect.Request[appv1.GetTimeEntryRequest]) (*connect.Response[appv1.GetTimeEntryResponse], error) {
	if err := requireID(req.Msg.Id, "id"); err != nil {
		return nil, err
	}
	if err := requireID(req.Msg.PeriodId, "period_id"); err != nil {
		return nil, err
	}
	item, err := s.service.GetTimeEntry(ctx, req.Msg.Id, req.Msg.PeriodId)
	if err != nil {
		return nil, mapServiceError(err)
	}
	return connect.NewResponse(&appv1.GetTimeEntryResponse{TimeEntry: toProtoTimeEntry(item)}), nil
}

func (s *ScheduleService) CreateTimeEntry(ctx context.Context, req *connect.Request[appv1.CreateTimeEntryRequest]) (*connect.Response[appv1.CreateTimeEntryResponse], error) {
	input, err := timeEntryInput(req.Msg.Input)
	if err != nil {
		return nil, err
	}
	item, serviceErr := s.service.CreateTimeEntry(ctx, input)
	if serviceErr != nil {
		return nil, mapServiceError(serviceErr)
	}
	return connect.NewResponse(&appv1.CreateTimeEntryResponse{PeriodId: item.PeriodID, Id: item.ID}), nil
}

func (s *ScheduleService) CreateGapTimeEntry(ctx context.Context, req *connect.Request[appv1.CreateGapTimeEntryRequest]) (*connect.Response[appv1.CreateGapTimeEntryResponse], error) {
	input, err := timeEntryInput(req.Msg.Input)
	if err != nil {
		return nil, err
	}
	item, serviceErr := s.service.CreateGapTimeEntry(ctx, input)
	if serviceErr != nil {
		return nil, mapServiceError(serviceErr)
	}
	return connect.NewResponse(&appv1.CreateGapTimeEntryResponse{PeriodId: item.PeriodID, Id: item.ID}), nil
}

func (s *ScheduleService) UpdateTimeEntry(ctx context.Context, req *connect.Request[appv1.UpdateTimeEntryRequest]) (*connect.Response[appv1.UpdateTimeEntryResponse], error) {
	if err := requireID(req.Msg.Id, "id"); err != nil {
		return nil, err
	}
	input, err := timeEntryInput(req.Msg.Input)
	if err != nil {
		return nil, err
	}
	item, serviceErr := s.service.UpdateTimeEntry(ctx, service.TimeEntryUpdateInput{ID: req.Msg.Id, TimeEntryInput: input})
	if serviceErr != nil {
		return nil, mapServiceError(serviceErr)
	}
	return connect.NewResponse(&appv1.UpdateTimeEntryResponse{PeriodId: item.PeriodID, Id: item.ID}), nil
}

func (s *ScheduleService) DeleteTimeEntry(ctx context.Context, req *connect.Request[appv1.DeleteTimeEntryRequest]) (*connect.Response[appv1.DeleteTimeEntryResponse], error) {
	if err := requireID(req.Msg.Id, "id"); err != nil {
		return nil, err
	}
	if err := requireID(req.Msg.PeriodId, "period_id"); err != nil {
		return nil, err
	}
	if err := s.service.DeleteTimeEntry(ctx, service.TimeEntryDeleteInput{ID: req.Msg.Id, PeriodID: req.Msg.PeriodId}); err != nil {
		return nil, mapServiceError(err)
	}
	return connect.NewResponse(&appv1.DeleteTimeEntryResponse{PeriodId: req.Msg.PeriodId, Id: req.Msg.Id}), nil
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

func (s *ScheduleService) ListGapEvidence(ctx context.Context, req *connect.Request[appv1.ListGapEvidenceRequest]) (*connect.Response[appv1.ListGapEvidenceResponse], error) {
	if req.Msg.Start == nil || req.Msg.End == nil {
		return nil, invalidArgument("start and end are required")
	}
	if err := req.Msg.Start.CheckValid(); err != nil {
		return nil, invalidArgument("start must be a valid timestamp")
	}
	if err := req.Msg.End.CheckValid(); err != nil {
		return nil, invalidArgument("end must be a valid timestamp")
	}
	items, err := s.service.ListGapEvidence(ctx, service.TimeWindow{Start: req.Msg.Start.AsTime(), End: req.Msg.End.AsTime()})
	if err != nil {
		return nil, mapServiceError(err)
	}
	out := make([]*appv1.GapEvidenceItem, len(items))
	for i, item := range items {
		out[i] = &appv1.GapEvidenceItem{
			Provider: item.Provider,
			Kind:     item.Kind,
			Summary:  item.Summary,
			Source:   item.Source,
		}
	}
	return connect.NewResponse(&appv1.ListGapEvidenceResponse{Items: out}), nil
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

func timeEntryInput(input *appv1.TimeEntryInput) (service.TimeEntryInput, error) {
	if input == nil {
		return service.TimeEntryInput{}, invalidArgument("input is required")
	}
	if err := requireID(input.PeriodId, "period_id"); err != nil {
		return service.TimeEntryInput{}, err
	}
	if input.Day == "" {
		return service.TimeEntryInput{}, invalidArgument("day is required")
	}
	if input.StartMinutes < 0 || input.EndMinutes > 24*60 || input.StartMinutes >= input.EndMinutes {
		return service.TimeEntryInput{}, invalidArgument("time entry minute range is invalid")
	}
	return service.TimeEntryInput{
		PeriodID:       input.PeriodId,
		Day:            input.Day,
		StartMinutes:   int(input.StartMinutes),
		EndMinutes:     int(input.EndMinutes),
		CategoryID:     input.CategoryId,
		Description:    input.Description,
		WorkType:       input.WorkType,
		ProjectID:      input.ProjectId,
		BillableStatus: input.BillableStatus,
	}, nil
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

func toProtoTimeEntry(item service.TimeEntry) *appv1.TimeEntry {
	return &appv1.TimeEntry{
		Id:              item.ID,
		PeriodId:        item.PeriodID,
		LocalWorkDate:   item.LocalWorkDate,
		Start:           item.Start,
		End:             item.End,
		DurationMinutes: int32(item.DurationMinutes),
		CategoryId:      item.CategoryID,
		Description:     item.Description,
		Attestation:     item.Attestation,
		Method:          item.Method,
		WorkType:        item.WorkType,
		ProjectId:       item.ProjectID,
		BillableStatus:  item.BillableStatus,
	}
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
