import {
  ALL_DAY_CHIP_HEIGHT,
  ALL_DAY_ROW_MIN_HEIGHT,
  ALL_DAY_ROW_PADDING,
  INITIAL_SCROLL_CONTEXT_MINUTES,
  TIMELINE_HOUR_HEIGHT,
  WORKING_START_MINUTES,
} from "@/lib/schedule";
import {
  MINUTES_PER_DAY,
  clamp,
  snapMinutes,
  type SchedulerCreateRequest,
  type SchedulerVisibleRange,
} from "@/lib/scheduler";

export const TIMELINE_MIN_HEIGHT = 760;
export const TIME_AXIS_COLUMN_WIDTH = 72;
export const DAY_COLUMN_MIN_WIDTH = 116;
export const DAY_HEADER_ROW_HEIGHT = 52;
export const ALL_DAY_CHIP_GAP = 4;
export const TIMELINE_MARK_STEP_MINUTES = 60;

export const NON_WORKING_HOURS_BACKGROUND =
  "repeating-linear-gradient(135deg, var(--muted) 0 2px, transparent 2px 12px), var(--background)";

export interface TimelinePointerRect {
  top: number;
  height: number;
}

export function computeAllDayRowHeight(
  allDayChipsByDay:
    | Map<unknown, readonly unknown[]>
    | Iterable<readonly unknown[]>,
) {
  let maxChipsPerDay = 0;
  const chipGroups =
    allDayChipsByDay instanceof Map
      ? allDayChipsByDay.values()
      : allDayChipsByDay;

  for (const chips of chipGroups) {
    maxChipsPerDay = Math.max(maxChipsPerDay, chips.length);
  }

  if (maxChipsPerDay === 0) {
    return 0;
  }

  const chipStackHeight =
    maxChipsPerDay * ALL_DAY_CHIP_HEIGHT +
    Math.max(0, maxChipsPerDay - 1) * ALL_DAY_CHIP_GAP;

  return Math.max(
    ALL_DAY_ROW_MIN_HEIGHT,
    ALL_DAY_ROW_PADDING * 2 + chipStackHeight,
  );
}

export function timelineDuration(visibleRange: SchedulerVisibleRange) {
  return visibleRange.endMinutes - visibleRange.startMinutes;
}

export function computeTimelineHeight(visibleRange: SchedulerVisibleRange) {
  const duration = timelineDuration(visibleRange);
  return Math.max((duration / 60) * TIMELINE_HOUR_HEIGHT, TIMELINE_MIN_HEIGHT);
}

export function minuteToTimelinePercent(
  minute: number,
  visibleRange: SchedulerVisibleRange,
) {
  const duration = timelineDuration(visibleRange);
  if (duration <= 0) {
    return 0;
  }

  return clamp(
    ((minute - visibleRange.startMinutes) / duration) * 100,
    0,
    100,
  );
}

export function buildTimelineMarks(
  visibleRange: SchedulerVisibleRange,
  stepMinutes = TIMELINE_MARK_STEP_MINUTES,
) {
  const marks: number[] = [];
  const step = Math.max(1, stepMinutes);

  for (
    let minute = visibleRange.startMinutes;
    minute <= visibleRange.endMinutes;
    minute += step
  ) {
    marks.push(minute);
  }

  return marks;
}

export function getTimeLabelClass(
  minute: number,
  visibleRange: SchedulerVisibleRange,
) {
  if (minute === visibleRange.startMinutes) {
    return "absolute right-3 translate-y-0 text-xs font-medium text-muted-foreground";
  }

  if (minute === visibleRange.endMinutes) {
    return "absolute right-3 -translate-y-full text-xs font-medium text-muted-foreground";
  }

  return "absolute right-3 -translate-y-2 text-xs font-medium text-muted-foreground";
}

export function buildHourGridBackground(visibleRange: SchedulerVisibleRange) {
  const duration = timelineDuration(visibleRange);
  const slotPercent = duration <= 0 ? 0 : (60 / duration) * 100;

  return `repeating-linear-gradient(to bottom, transparent 0, transparent calc(${slotPercent}% - 1px), var(--border) calc(${slotPercent}% - 1px), var(--border) ${slotPercent}%)`;
}

export function pointerClientYToSnappedMinute(
  clientY: number,
  rect: TimelinePointerRect,
  visibleRange: SchedulerVisibleRange,
  slotMinutes: number,
) {
  const percent =
    rect.height <= 0 ? 0 : clamp((clientY - rect.top) / rect.height, 0, 1);
  const rawMinutes =
    visibleRange.startMinutes + percent * timelineDuration(visibleRange);

  return snapMinutes(rawMinutes, slotMinutes);
}

export function buildBackgroundCreateRequest({
  day,
  clientY,
  rect,
  visibleRange,
  slotMinutes,
  createDurationMinutes,
}: {
  day: string;
  clientY: number;
  rect: TimelinePointerRect;
  visibleRange: SchedulerVisibleRange;
  slotMinutes: number;
  createDurationMinutes: number;
}): SchedulerCreateRequest {
  const startMinutes = clamp(
    pointerClientYToSnappedMinute(clientY, rect, visibleRange, slotMinutes),
    0,
    MINUTES_PER_DAY - createDurationMinutes,
  );

  return {
    day,
    startMinutes,
    endMinutes: startMinutes + createDurationMinutes,
  };
}

export function computeInitialTimelineScrollTop({
  visibleRange,
  workingStartMinutes = WORKING_START_MINUTES,
  contextMinutes = INITIAL_SCROLL_CONTEXT_MINUTES,
  timelineHeight,
}: {
  visibleRange: SchedulerVisibleRange;
  workingStartMinutes?: number;
  contextMinutes?: number;
  timelineHeight: number;
}) {
  const duration = timelineDuration(visibleRange);
  if (duration <= 0) {
    return 0;
  }

  const initialMinute = Math.max(
    visibleRange.startMinutes,
    workingStartMinutes - contextMinutes,
  );

  return (
    ((initialMinute - visibleRange.startMinutes) / duration) * timelineHeight
  );
}
