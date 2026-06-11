import {
  DEFAULT_SCHEDULER_CONFIG,
  type SchedulerConfig,
  type SchedulerItem,
  type SchedulerVisibleRange,
} from "./types";

export const MINUTES_PER_DAY = 24 * 60;

export function normalizeConfig(config?: Partial<SchedulerConfig>): SchedulerConfig {
  const merged = {
    ...DEFAULT_SCHEDULER_CONFIG,
    ...config,
  };

  const slotMinutes = clamp(Math.floor(merged.slotMinutes) || 1, 1, MINUTES_PER_DAY);
  const scheduleRange = normalizeMinuteRange(
    merged.scheduleStartMinutes,
    merged.scheduleEndMinutes,
    slotMinutes,
  );
  const workingRange = normalizeMinuteRange(
    merged.workingStartMinutes,
    merged.workingEndMinutes,
    1,
  );

  return {
    ...merged,
    slotMinutes,
    scheduleStartMinutes: scheduleRange.startMinutes,
    scheduleEndMinutes: scheduleRange.endMinutes,
    workingStartMinutes: workingRange.startMinutes,
    workingEndMinutes: workingRange.endMinutes,
    minDurationMinutes: clamp(merged.minDurationMinutes, 1, MINUTES_PER_DAY),
    createDurationMinutes: clamp(merged.createDurationMinutes, 1, MINUTES_PER_DAY),
    maxDays: Math.max(1, merged.maxDays),
    dragThresholdPx: Math.max(0, merged.dragThresholdPx),
  };
}

export function clamp(value: number, min: number, max: number) {
  return Math.min(Math.max(value, min), max);
}

function finiteMinute(value: number, fallback: number) {
  return Number.isFinite(value) ? Math.floor(value) : fallback;
}

function normalizeMinuteRange(start: number, end: number, minDuration: number) {
  const duration = clamp(Math.floor(minDuration) || 1, 1, MINUTES_PER_DAY);
  const startMinutes = clamp(
    finiteMinute(start, 0),
    0,
    MINUTES_PER_DAY - duration,
  );
  const endMinutes = clamp(
    finiteMinute(end, startMinutes + duration),
    startMinutes + duration,
    MINUTES_PER_DAY,
  );

  return { startMinutes, endMinutes };
}

export function snapMinutes(value: number, slotMinutes: number) {
  return clamp(Math.round(value / slotMinutes) * slotMinutes, 0, MINUTES_PER_DAY);
}

export function snapMinutesDown(value: number, slotMinutes: number) {
  return clamp(Math.floor(value / slotMinutes) * slotMinutes, 0, MINUTES_PER_DAY);
}

export function snapMinutesUp(value: number, slotMinutes: number) {
  return clamp(Math.ceil(value / slotMinutes) * slotMinutes, 0, MINUTES_PER_DAY);
}

export function formatMinutes(totalMinutes: number) {
  const minutes = clamp(totalMinutes, 0, MINUTES_PER_DAY);
  const hours = Math.floor(minutes / 60);
  const mins = minutes % 60;
  return `${String(hours).padStart(2, "0")}:${String(mins).padStart(2, "0")}`;
}

export function addDays(date: string, offset: number) {
  const [year, month, day] = date.split("-").map(Number);
  const next = new Date(Date.UTC(year, month - 1, day + offset));
  return next.toISOString().slice(0, 10);
}

export function calculateVisibleRange<TMetadata>(
  items: SchedulerItem<TMetadata>[],
  config: SchedulerConfig,
): SchedulerVisibleRange {
  const range = items.reduce(
    (next, item) => ({
      startMinutes: Math.min(next.startMinutes, item.startMinutes),
      endMinutes: Math.max(next.endMinutes, item.endMinutes),
    }),
    {
      startMinutes: config.scheduleStartMinutes,
      endMinutes: config.scheduleEndMinutes,
    },
  );

  // Floor the start and ceil the end so callers can opt into a narrower range
  // without clipping off-slot items.
  const startMinutes = Math.min(
    config.scheduleStartMinutes,
    snapMinutesDown(range.startMinutes, config.slotMinutes),
  );
  const endMinutes = Math.max(
    config.scheduleEndMinutes,
    snapMinutesUp(range.endMinutes, config.slotMinutes),
  );

  return {
    startMinutes: clamp(startMinutes, 0, MINUTES_PER_DAY - config.slotMinutes),
    endMinutes: clamp(endMinutes, config.slotMinutes, MINUTES_PER_DAY),
  };
}

export function minutesToPercent(minutes: number, range: SchedulerVisibleRange) {
  const duration = range.endMinutes - range.startMinutes;
  if (duration <= 0) {
    return 0;
  }
  return ((minutes - range.startMinutes) / duration) * 100;
}

export function durationToPercent(minutes: number, range: SchedulerVisibleRange) {
  const duration = range.endMinutes - range.startMinutes;
  if (duration <= 0) {
    return 0;
  }
  return (minutes / duration) * 100;
}
