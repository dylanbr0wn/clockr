import type {
  Category,
  DayTimeline,
  Event,
  EventCategoryOverlay,
  GapFill,
  Period,
  ReviewItem,
  TzSegment,
} from "@/lib/api";
import {
  START_DATE,
  buildAllDayChipsByDay,
  buildDays,
  buildEventCategoryOverlayMap,
  eventToSchedulerItem,
  gapFillToSchedulerItem,
  gapTimelineToOverlays,
  periodContainsDate,
  periodDayCount,
  resolveEventCategoryId,
  type AllDayChip,
  type ScheduleDay,
  type ScheduleGapOverlay,
  type ScheduleItem,
  type SchedulePlacement,
} from "@/lib/schedule";
import { calculateTotals, mergePeriods } from "./useSchedulePage.helpers";
import type { EditableScheduleEvent, ScheduleViewDayCount } from "./schedulePage.types";

export interface BuildSchedulePageDerivedArgs {
  selectedPeriodId: number | null;
  viewDayCount: ScheduleViewDayCount;
  today: string;
  persistedPeriods: Period[];
  currentPeriod: Period | null;
  categories: Category[];
  events: Event[];
  eventCategoryOverlays: EventCategoryOverlay[];
  gapFills: GapFill[];
  gapTimeline: DayTimeline[];
  reviewItems: ReviewItem[];
  tzSegments: TzSegment[];
  draftPlacements: Record<string, SchedulePlacement>;
  pendingCreate: {
    day: string;
    startMinutes: number;
    endMinutes: number;
  } | null;
  editingItemId: string | null;
}

export interface SchedulePageDerivedData {
  periods: Period[];
  activePeriod: Period | null;
  activePeriodId: number | undefined;
  categoriesById: Map<number, Category>;
  days: ScheduleDay[];
  visibleDayCount: number;
  allDayChipsByDay: Map<string, AllDayChip[]>;
  items: ScheduleItem[];
  visibleGaps: ScheduleGapOverlay[];
  resettableDays: ReadonlySet<string>;
  gapFillsByItemId: Map<string, GapFill>;
  totals: Record<string, number>;
  editingEvent: EditableScheduleEvent | null;
}

export function buildSchedulePageDerived({
  selectedPeriodId,
  viewDayCount,
  today,
  persistedPeriods,
  currentPeriod,
  categories,
  events,
  eventCategoryOverlays,
  gapFills,
  gapTimeline,
  reviewItems,
  tzSegments,
  draftPlacements,
  pendingCreate,
  editingItemId,
}: BuildSchedulePageDerivedArgs): SchedulePageDerivedData {
  const periods = mergePeriods(persistedPeriods, currentPeriod);
  const activePeriod = resolveActivePeriod({
    selectedPeriodId,
    currentPeriod,
    periods,
    today,
  });
  const activePeriodId = activePeriod?.id;

  const periodVisibleDayCount = activePeriod ? periodDayCount(activePeriod) : viewDayCount;
  const visibleDayCount = Math.min(periodVisibleDayCount, viewDayCount);
  const days = buildDays(activePeriod?.startDate ?? START_DATE, visibleDayCount);
  const visibleDaySet = new Set(days.map((day) => day.date));

  const categoriesById = new Map(categories.map((category) => [category.id, category]));
  const overlaysByKey = buildEventCategoryOverlayMap(eventCategoryOverlays);
  const gapFillsByItemId = new Map(
    gapFills.map((gapFill) => [`gap-fill-${gapFill.id}`, gapFill]),
  );
  const reviewItemsByEventId = new Map(
    reviewItems
      .filter((item) => typeof item.eventId === "number")
      .map((item) => [item.eventId as number, { reviewItemId: item.id, kind: item.kind }]),
  );

  const allDayChipsByDay = buildAllDayChipsByDay(
    events,
    visibleDaySet,
    categoriesById,
    overlaysByKey,
    reviewItemsByEventId,
  );
  const items = [
    ...events
      .map((event) =>
        eventToSchedulerItem(
          event,
          tzSegments,
          categoriesById,
          resolveEventCategoryId(event, overlaysByKey),
          draftPlacements[`event-${event.id}`],
          reviewItemsByEventId.get(event.id),
        ),
      )
      .filter((item): item is ScheduleItem => item !== null),
    ...gapFills
      .map((gapFill) =>
        gapFillToSchedulerItem(
          gapFill,
          categoriesById,
          tzSegments,
          draftPlacements[`gap-fill-${gapFill.id}`],
        ),
      )
      .filter((item): item is ScheduleItem => item !== null),
  ];

  const visibleGaps = gapTimelineToOverlays(gapTimeline, visibleDaySet, tzSegments);
  const resettableDays = new Set(
    gapFills
      .filter((gapFill) => gapFill.source === "manual")
      .map((gapFill) => gapFill.day),
  );

  const editingEvent = resolveEditingEvent({
    pendingCreate,
    activePeriodId,
    editingItemId,
    gapFillsByItemId,
    items,
  });

  return {
    periods,
    activePeriod,
    activePeriodId,
    categoriesById,
    days,
    visibleDayCount,
    allDayChipsByDay,
    items,
    visibleGaps,
    resettableDays,
    gapFillsByItemId,
    totals: calculateTotals(items),
    editingEvent,
  };
}

export function resolveActivePeriod({
  selectedPeriodId,
  currentPeriod,
  periods,
  today,
}: {
  selectedPeriodId: number | null;
  currentPeriod: Period | null;
  periods: Period[];
  today: string;
}): Period | null {
  return (
    periods.find((period) => period.id === selectedPeriodId) ??
    currentPeriod ??
    periods.find((period) => periodContainsDate(period, today)) ??
    periods[0] ??
    null
  );
}

function resolveEditingEvent({
  pendingCreate,
  activePeriodId,
  editingItemId,
  gapFillsByItemId,
  items,
}: {
  pendingCreate: BuildSchedulePageDerivedArgs["pendingCreate"];
  activePeriodId: number | undefined;
  editingItemId: string | null;
  gapFillsByItemId: Map<string, GapFill>;
  items: ScheduleItem[];
}): EditableScheduleEvent | null {
  if (pendingCreate && activePeriodId) {
    return {
      id: "__new__",
      periodId: activePeriodId,
      day: pendingCreate.day,
      startMinutes: pendingCreate.startMinutes,
      endMinutes: pendingCreate.endMinutes,
      category: "Unassigned",
      note: "",
      isNew: true,
    };
  }

  if (!editingItemId) {
    return null;
  }

  const gapFill = gapFillsByItemId.get(editingItemId);
  const item = items.find((candidate) => candidate.id === editingItemId);

  if (!gapFill || !item) {
    return null;
  }

  return {
    id: editingItemId,
    gapFillId: gapFill.id,
    periodId: gapFill.periodId,
    day: item.day,
    startMinutes: item.startMinutes,
    endMinutes: item.endMinutes,
    category: item.metadata?.category ?? "Unassigned",
    categoryId: gapFill.categoryId,
    note: gapFill.note ?? "",
  };
}
