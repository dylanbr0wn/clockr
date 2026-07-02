import type {
  SchedulerDay,
  SchedulerItem,
  SchedulerChange,
} from "@/lib/scheduler";

export type ScheduleKind = "calendar" | "gap" | "manual" | "review" | "uncovered";

export interface ScheduleMetadata {
  title: string;
  category: string;
  kind: ScheduleKind;
  /** UTC RFC3339 bounds for uncovered gaps (used by AI suggest). */
  gapWindowStart?: string;
  gapWindowEnd?: string;
}

export interface ScheduleDayMetadata {
  isWeekend: boolean;
}

export type ScheduleItem = SchedulerItem<ScheduleMetadata>;
export type ScheduleDay = SchedulerDay<ScheduleDayMetadata>;
export type ScheduleChange = SchedulerChange<ScheduleMetadata>;
export type SchedulePlacement = Pick<
  ScheduleItem,
  "day" | "endMinutes" | "startMinutes"
>;
