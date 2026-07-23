import type {
  Category,
  DayTimeline,
  Event,
  EventCategoryOverlay,
  TimeEntry,
  Period,
  ReviewDecision,
  TzSegment,
} from "@/lib/api";
import {
  START_DATE,
  buildDays,
  DEFAULT_BILLABLE_STATUS,
  DEFAULT_WORK_TYPE,
  periodContainsDate,
  periodDayCount,
  projectSchedulePeriod,
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
  timeEntries: TimeEntry[];
  gapTimeline: DayTimeline[];
  reviewDecisions: ReviewDecision[];
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
  timeEntriesByItemId: Map<string, TimeEntry>;
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
  timeEntries,
  gapTimeline,
  reviewDecisions,
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

  const projected = projectSchedulePeriod({
    events,
    eventCategoryOverlays,
    timeEntries,
    gapTimeline,
    reviewDecisions,
    tzSegments,
    categories,
    visibleDays: visibleDaySet,
    draftPlacements,
  });

  const editingEvent = resolveEditingEvent({
    pendingCreate,
    activePeriodId,
    editingItemId,
    timeEntriesByItemId: projected.timeEntriesByItemId,
    items: projected.items,
  });

  return {
    periods,
    activePeriod,
    activePeriodId,
    categoriesById: projected.categoriesById,
    days,
    visibleDayCount,
    allDayChipsByDay: projected.allDayChipsByDay,
    items: projected.items,
    visibleGaps: projected.visibleGaps,
    resettableDays: projected.resettableDays,
    timeEntriesByItemId: projected.timeEntriesByItemId,
    totals: calculateTotals(projected.items),
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
  timeEntriesByItemId,
  items,
}: {
  pendingCreate: BuildSchedulePageDerivedArgs["pendingCreate"];
  activePeriodId: number | undefined;
  editingItemId: string | null;
  timeEntriesByItemId: Map<string, TimeEntry>;
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
      description: "",
      workType: DEFAULT_WORK_TYPE,
      billableStatus: DEFAULT_BILLABLE_STATUS,
      isNew: true,
    };
  }

  if (!editingItemId) {
    return null;
  }

  const timeEntry = timeEntriesByItemId.get(editingItemId);
  const item = items.find((candidate) => candidate.id === editingItemId);

  if (!timeEntry || !item || timeEntry.attestation === "draft") {
    return null;
  }

  return {
    id: editingItemId,
    timeEntryId: timeEntry.id,
    periodId: timeEntry.periodId,
    day: item.day,
    startMinutes: item.startMinutes,
    endMinutes: item.endMinutes,
    category: item.metadata?.category ?? "Unassigned",
    categoryId: timeEntry.categoryId,
    note: timeEntry.description ?? "",
    description: timeEntry.description ?? "",
    workType: timeEntry.workType,
    projectId: timeEntry.projectId,
    billableStatus: timeEntry.billableStatus,
  };
}
