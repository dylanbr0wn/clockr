import type {
  Category,
  DayTimeline,
  Event as ShietEvent,
  EventCategoryOverlay,
  GapFill,
  Interval,
  Period,
  TzSegment,
} from "@/lib/api";
import {
  SCHEDULE_END_MINUTES,
  SCHEDULE_START_MINUTES,
} from "./constants";
import { addDays } from "@/lib/scheduler";
import {
  activeTimeZoneForDay,
  toDate,
  zonedDateTimeParts,
  zonedPosition,
} from "./timezone";
import type {
  AllDayChip,
  AllDaySpanPosition,
  ScheduleGapOverlay,
  ScheduleItem,
  SchedulePlacement,
} from "./types";

export interface EventReviewState {
  reviewItemId: number;
  kind: string;
}

export function categoryName(
  categoryId: number | undefined,
  categoriesById: Map<number, Category>,
) {
  if (typeof categoryId !== "number") {
    return "Unassigned";
  }

  return categoriesById.get(categoryId)?.name ?? "Unassigned";
}

export function resolveCategoryColor(
  categoryId: number | undefined,
  categoriesById: Map<number, Category>,
) {
  if (typeof categoryId !== "number") {
    return undefined;
  }

  return categoriesById.get(categoryId)?.color;
}

export function eventCategoryOverlayKey(
  provider: string,
  externalId: string,
  instanceId = "",
) {
  return `${provider}|${externalId}|${instanceId}`;
}

export function buildEventCategoryOverlayMap(
  overlays: EventCategoryOverlay[],
) {
  return new Map(
    overlays.map((overlay) => [
      eventCategoryOverlayKey(
        overlay.provider,
        overlay.externalId,
        overlay.instanceId ?? "",
      ),
      overlay.categoryId,
    ]),
  );
}

export function resolveEventCategoryId(
  event: ShietEvent,
  overlaysByKey: ReadonlyMap<string, number>,
) {
  return overlaysByKey.get(
    eventCategoryOverlayKey(
      event.provider,
      event.externalId,
      event.instanceId ?? "",
    ),
  );
}

export function periodContainsDate(period: Period, day: string) {
  return period.startDate <= day && day <= period.endDate;
}

export function applyPlacement(
  item: ScheduleItem,
  placement?: SchedulePlacement,
) {
  if (!placement) {
    return item;
  }

  return {
    ...item,
    ...placement,
  };
}

export function eventToSchedulerItem(
  event: ShietEvent,
  tzSegments: TzSegment[],
  categoriesById: Map<number, Category>,
  categoryId: number | undefined,
  placement?: SchedulePlacement,
  reviewState?: EventReviewState,
): ScheduleItem | null {
  const resolvedCategoryId = reviewState ? undefined : categoryId;
  const metadata = {
    title: event.title || "Untitled event",
    category: reviewState
      ? "Needs review"
      : categoryName(resolvedCategoryId, categoriesById),
    categoryId: resolvedCategoryId,
    categoryColor: reviewState
      ? undefined
      : resolveCategoryColor(resolvedCategoryId, categoriesById),
    kind: reviewState ? "review" : "calendar",
    reviewItemId: reviewState?.reviewItemId,
    reviewKind: reviewState?.kind,
  } as const;

  if (event.allDay && event.startDate) {
    return applyPlacement(
      {
        id: `event-${event.id}`,
        day: event.startDate,
        startMinutes: SCHEDULE_START_MINUTES,
        endMinutes: SCHEDULE_END_MINUTES,
        disabled: true,
        metadata: {
          ...metadata,
          isAllDay: true,
        },
      },
      placement,
    );
  }

  const start = zonedPosition(event.start, tzSegments);
  const end = zonedPosition(event.end, tzSegments);

  if (!start || !end) {
    return null;
  }

  const endMinutes =
    end.day === start.day ? end.minutes : SCHEDULE_END_MINUTES;

  return applyPlacement(
    {
      id: `event-${event.id}`,
      day: start.day,
      startMinutes: start.minutes,
      endMinutes: Math.max(start.minutes + 15, endMinutes),
      disabled: true,
      metadata,
    },
    placement,
  );
}

export function expandAllDayEventDays(event: ShietEvent): string[] {
  if (!event.startDate) {
    return [];
  }

  const start = event.startDate;
  const exclusiveEnd = event.endDate ?? addDays(start, 1);
  const days: string[] = [];

  for (let day = start; day < exclusiveEnd; day = addDays(day, 1)) {
    days.push(day);
  }

  return days.length > 0 ? days : [start];
}

function allDaySpanPosition(
  day: string,
  spanDays: string[],
): AllDaySpanPosition {
  if (spanDays.length <= 1) {
    return "single";
  }

  if (day === spanDays[0]) {
    return "start";
  }

  if (day === spanDays[spanDays.length - 1]) {
    return "end";
  }

  return "middle";
}

export function buildAllDayChipsByDay(
  events: ShietEvent[],
  visibleDays: ReadonlySet<string>,
  categoriesById: Map<number, Category>,
  overlaysByKey: ReadonlyMap<string, number>,
  reviewByEventId: ReadonlyMap<number, EventReviewState>,
): Map<string, AllDayChip[]> {
  const chipsByDay = new Map<string, AllDayChip[]>();

  for (const event of events) {
    if (!event.allDay || !event.startDate) {
      continue;
    }

    const reviewState = reviewByEventId.get(event.id);
    const spanDays = expandAllDayEventDays(event);
    const title = event.title || "Untitled event";
    const kind = reviewState ? "review" : "calendar";
    const categoryId = reviewState
      ? undefined
      : resolveEventCategoryId(event, overlaysByKey);
    const category = reviewState
      ? "Needs review"
      : categoryName(categoryId, categoriesById);
    const categoryColorValue = reviewState
      ? undefined
      : resolveCategoryColor(categoryId, categoriesById);

    for (const day of spanDays) {
      if (!visibleDays.has(day)) {
        continue;
      }

      const chip: AllDayChip = {
        id: `event-${event.id}@${day}`,
        eventId: event.id,
        day,
        title,
        category,
        categoryId,
        categoryColor: categoryColorValue,
        kind,
        reviewItemId: reviewState?.reviewItemId,
        reviewKind: reviewState?.kind,
        allDaySpan: allDaySpanPosition(day, spanDays),
      };
      const dayChips = chipsByDay.get(day) ?? [];
      dayChips.push(chip);
      chipsByDay.set(day, dayChips);
    }
  }

  return chipsByDay;
}

export function gapFillToSchedulerItem(
  gapFill: GapFill,
  categoriesById: Map<number, Category>,
  tzSegments: TzSegment[],
  placement?: SchedulePlacement,
): ScheduleItem | null {
  const timeZone = activeTimeZoneForDay(gapFill.day, tzSegments);
  const startDate = toDate(gapFill.start);
  const endDate = toDate(gapFill.end);

  if (!startDate || !endDate) {
    return null;
  }

  const start = zonedDateTimeParts(startDate, timeZone);
  const end = zonedDateTimeParts(endDate, timeZone);
  const startMinutes = start.minutes;
  const endMinutes =
    end.day === start.day ? end.minutes : SCHEDULE_END_MINUTES;
  const category = categoryName(gapFill.categoryId, categoriesById);
  const kind = gapFill.source === "manual" ? "manual" : "gap";

  return applyPlacement(
    {
      id: `gap-fill-${gapFill.id}`,
      day: gapFill.day || start.day,
      startMinutes,
      endMinutes: Math.max(startMinutes + 15, endMinutes),
      metadata: {
        title: gapFill.note || category,
        category,
        categoryId: gapFill.categoryId,
        categoryColor: resolveCategoryColor(gapFill.categoryId, categoriesById),
        kind,
      },
    },
    placement,
  );
}

function intervalUtcValue(value: string | Date | undefined) {
  if (!value) {
    return null;
  }

  if (value instanceof Date) {
    return value.toISOString();
  }

  const date = toDate(value);
  return date?.toISOString() ?? null;
}

export function gapIntervalToOverlay(
  day: string,
  interval: Interval,
  tzSegments: TzSegment[],
): ScheduleGapOverlay | null {
  const startIso = intervalUtcValue(interval.start);
  const endIso = intervalUtcValue(interval.end);
  const startDate = toDate(startIso ?? undefined);
  const endDate = toDate(endIso ?? undefined);

  if (!startDate || !endDate || !startIso || !endIso) {
    return null;
  }

  const timeZone = activeTimeZoneForDay(day, tzSegments);
  const start = zonedDateTimeParts(startDate, timeZone);
  const end = zonedDateTimeParts(endDate, timeZone);
  const startMinutes = start.minutes;
  const endMinutes =
    end.day === start.day ? end.minutes : SCHEDULE_END_MINUTES;

  return {
    id: `gap-${day}-${startIso}-${endIso}`,
    day,
    startMinutes,
    endMinutes: Math.max(startMinutes + 15, endMinutes),
    gapWindowStart: startIso,
    gapWindowEnd: endIso,
  };
}

export function gapTimelineToOverlays(
  timelines: DayTimeline[],
  visibleDays: ReadonlySet<string>,
  tzSegments: TzSegment[],
): ScheduleGapOverlay[] {
  return timelines.flatMap((timeline) => {
    if (!visibleDays.has(timeline.date)) {
      return [];
    }

    return timeline.gaps
      .map((gap) => gapIntervalToOverlay(timeline.date, gap, tzSegments))
      .filter((gap): gap is ScheduleGapOverlay => gap !== null);
  });
}
