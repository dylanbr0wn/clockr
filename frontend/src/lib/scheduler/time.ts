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

  return {
    ...merged,
    slotMinutes,
    minDurationMinutes: clamp(merged.minDurationMinutes, 1, MINUTES_PER_DAY),
    createDurationMinutes: clamp(merged.createDurationMinutes, 1, MINUTES_PER_DAY),
    maxDays: Math.max(1, merged.maxDays),
    dragThresholdPx: Math.max(0, merged.dragThresholdPx),
  };
}

export function clamp(value: number, min: number, max: number) {
  return Math.min(Math.max(value, min), max);
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
      startMinutes: config.workingStartMinutes,
      endMinutes: config.workingEndMinutes,
    },
  );

  // Floor the start and ceil the end so items at the edges are never clipped.
  const startMinutes = snapMinutesDown(range.startMinutes, config.slotMinutes);
  const endMinutes = snapMinutesUp(range.endMinutes, config.slotMinutes);

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
