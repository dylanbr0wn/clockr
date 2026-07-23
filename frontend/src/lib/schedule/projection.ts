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
  TimeEntry,
  ReviewDecision,
  TzSegment,
} from "@/lib/api";
import { buildTimeEntriesByItemId, eventItemId, timeEntryItemId } from "./ids";
import {
  buildAllDayChipsByDay,
  buildEventCategoryOverlayMap,
  eventToSchedulerItem,
  timeEntryToSchedulerItem,
  gapTimelineToOverlays,
  resolveEventCategoryId,
  type EventReviewState,
} from "./mappers";
import { toDate } from "./timezone";
import type {
  AllDayChip,
  ScheduleGapOverlay,
  ScheduleItem,
  SchedulePlacement,
} from "./types";

export interface ProjectSchedulePeriodArgs {
  events: Event[];
  eventCategoryOverlays: EventCategoryOverlay[];
  timeEntries: TimeEntry[];
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
  timeEntriesByItemId: Map<string, TimeEntry>;
  reviewDecisionsByEventId: Map<number, EventReviewState>;
}

const CALENDAR_LINKED_METHODS = new Set([
  "calendar_import",
  "calendar_convert",
]);

function intervalsOverlap(
  startA: Date,
  endA: Date,
  startB: Date,
  endB: Date,
) {
  return startA.getTime() < endB.getTime() && endA.getTime() > startB.getTime();
}

/** Timed calendar events covered by a live calendar-linked TimeEntry stay off the canvas. */
export function isTimedEventCoveredByLinkedEntry(
  event: Event,
  linkedEntries: TimeEntry[],
): boolean {
  if (event.allDay) {
    return false;
  }
  const start = toDate(event.start);
  const end = toDate(event.end);
  if (!start || !end) {
    return false;
  }

  return linkedEntries.some((entry) => {
    const entryStart = toDate(entry.start);
    const entryEnd = toDate(entry.end);
    if (!entryStart || !entryEnd) {
      return false;
    }
    return intervalsOverlap(start, end, entryStart, entryEnd);
  });
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

export function buildResettableDays(timeEntries: TimeEntry[]): ReadonlySet<string> {
  return new Set(
    timeEntries
      .filter((timeEntry) => !timeEntry.method)
      .map((timeEntry) => timeEntry.localWorkDate),
  );
}

export function projectSchedulePeriod({
  events,
  eventCategoryOverlays,
  timeEntries,
  gapTimeline,
  reviewDecisions,
  tzSegments,
  categories,
  visibleDays,
  draftPlacements,
}: ProjectSchedulePeriodArgs): ProjectedSchedulePeriod {
  const categoriesById = new Map(categories.map((category) => [category.id, category]));
  const overlaysByKey = buildEventCategoryOverlayMap(eventCategoryOverlays);
  const timeEntriesByItemId = buildTimeEntriesByItemId(timeEntries);
  const reviewDecisionsByEventId = buildReviewStateByEventId(reviewDecisions);
  const linkedCalendarEntries = timeEntries.filter(
    (entry) =>
      entry.attestation !== "dismissed" &&
      typeof entry.method === "string" &&
      CALENDAR_LINKED_METHODS.has(entry.method),
  );

  const allDayChipsByDay = buildAllDayChipsByDay(
    events,
    visibleDays,
    categoriesById,
    overlaysByKey,
    reviewDecisionsByEventId,
  );

  const items = [
    ...events
      .filter(
        (event) =>
          !isTimedEventCoveredByLinkedEntry(event, linkedCalendarEntries),
      )
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
    ...timeEntries
      .map((timeEntry) =>
        timeEntryToSchedulerItem(
          timeEntry,
          categoriesById,
          tzSegments,
          draftPlacements[timeEntryItemId(timeEntry.id)],
        ),
      )
      .filter((item): item is ScheduleItem => item !== null),
  ];

  const visibleGaps = gapTimelineToOverlays(gapTimeline, visibleDays, tzSegments);
  const resettableDays = buildResettableDays(timeEntries);

  return {
    categoriesById,
    items,
    allDayChipsByDay,
    visibleGaps,
    resettableDays,
    timeEntriesByItemId,
    reviewDecisionsByEventId,
  };
}
