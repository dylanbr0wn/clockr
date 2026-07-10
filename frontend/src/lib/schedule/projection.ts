/**
 * Schedule projection concentrates shiet schedule policy for the timeline.
 *
 * Intentional remaining leakage (not projected here):
 * - Sidebar / NeedsAttentionCard consume raw ReviewDecision[] for the review queue UI.
 * - Hover-gap highlight state lives in ScheduleTimeline (pointer UI, not schedule policy).
 * - ScheduleKind remains on metadata/chips for presentation styling via scheduleItemPresentation.
 */
import type {
  Category,
  DayTimeline,
  Event,
  EventCategoryOverlay,
  GapFill,
  ReviewDecision,
  TzSegment,
} from "@/lib/api";
import { buildGapFillsByItemId, eventItemId, gapFillItemId } from "./ids";
import {
  buildAllDayChipsByDay,
  buildEventCategoryOverlayMap,
  eventToSchedulerItem,
  gapFillToSchedulerItem,
  gapTimelineToOverlays,
  resolveEventCategoryId,
  type EventReviewState,
} from "./mappers";
import type {
  AllDayChip,
  ScheduleGapOverlay,
  ScheduleItem,
  SchedulePlacement,
} from "./types";

export interface ProjectSchedulePeriodArgs {
  events: Event[];
  eventCategoryOverlays: EventCategoryOverlay[];
  gapFills: GapFill[];
  gapTimeline: DayTimeline[];
  reviewDecisions: ReviewDecision[];
  tzSegments: TzSegment[];
  categories: Category[];
  visibleDays: ReadonlySet<string>;
  draftPlacements: Record<string, SchedulePlacement>;
}

export interface ProjectedSchedulePeriod {
  categoriesById: Map<number, Category>;
  items: ScheduleItem[];
  allDayChipsByDay: Map<string, AllDayChip[]>;
  visibleGaps: ScheduleGapOverlay[];
  resettableDays: ReadonlySet<string>;
  gapFillsByItemId: Map<string, GapFill>;
  reviewDecisionsByEventId: Map<number, EventReviewState>;
}

export function buildReviewStateByEventId(
  reviewDecisions: ReviewDecision[],
): Map<number, EventReviewState> {
  return new Map(
    reviewDecisions
      .filter((decision) => typeof decision.eventId === "number")
      .map((decision) => [
        decision.eventId as number,
        { reviewItemId: decision.id, kind: decision.kind },
      ]),
  );
}

export function buildResettableDays(gapFills: GapFill[]): ReadonlySet<string> {
  return new Set(
    gapFills
      .filter((gapFill) => gapFill.source === "manual")
      .map((gapFill) => gapFill.day),
  );
}

export function projectSchedulePeriod({
  events,
  eventCategoryOverlays,
  gapFills,
  gapTimeline,
  reviewDecisions,
  tzSegments,
  categories,
  visibleDays,
  draftPlacements,
}: ProjectSchedulePeriodArgs): ProjectedSchedulePeriod {
  const categoriesById = new Map(categories.map((category) => [category.id, category]));
  const overlaysByKey = buildEventCategoryOverlayMap(eventCategoryOverlays);
  const gapFillsByItemId = buildGapFillsByItemId(gapFills);
  const reviewDecisionsByEventId = buildReviewStateByEventId(reviewDecisions);

  const allDayChipsByDay = buildAllDayChipsByDay(
    events,
    visibleDays,
    categoriesById,
    overlaysByKey,
    reviewDecisionsByEventId,
  );

  const items = [
    ...events
      .map((event) =>
        eventToSchedulerItem(
          event,
          tzSegments,
          categoriesById,
          resolveEventCategoryId(event, overlaysByKey),
          draftPlacements[eventItemId(event.id)],
          reviewDecisionsByEventId.get(event.id),
        ),
      )
      .filter((item): item is ScheduleItem => item !== null),
    ...gapFills
      .map((gapFill) =>
        gapFillToSchedulerItem(
          gapFill,
          categoriesById,
          tzSegments,
          draftPlacements[gapFillItemId(gapFill.id)],
        ),
      )
      .filter((item): item is ScheduleItem => item !== null),
  ];

  const visibleGaps = gapTimelineToOverlays(gapTimeline, visibleDays, tzSegments);
  const resettableDays = buildResettableDays(gapFills);

  return {
    categoriesById,
    items,
    allDayChipsByDay,
    visibleGaps,
    resettableDays,
    gapFillsByItemId,
    reviewDecisionsByEventId,
  };
}
