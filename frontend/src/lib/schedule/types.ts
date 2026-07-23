import type {
  SchedulerDay,
  SchedulerItem,
  SchedulerChange,
} from "@/lib/scheduler";

export type ScheduleKind = "calendar" | "gap" | "manual" | "review";

export type AllDaySpanPosition = "single" | "start" | "middle" | "end";

export interface ScheduleMetadata {
  title: string;
  category: string;
  categoryId?: number;
  categoryColor?: string;
  kind: ScheduleKind;
  /** TimeEntry attestation; payable totals use `confirmed` only. */
  attestation?: string;
  reviewItemId?: number;
  reviewKind?: string;
  isAllDay?: boolean;
  allDaySpan?: AllDaySpanPosition;
  /** Gap-fills: drag, resize, edit, duplicate, remove. */
  mutable?: boolean;
  /** Calendar events/chips: exclude from period. */
  excludable?: boolean;
  /** Review items/chips: click opens review queue. */
  opensReviewQueue?: boolean;
}

export interface AllDayChip {
  id: string;
  eventId: number;
  day: string;
  title: string;
  category: string;
  categoryId?: number;
  categoryColor?: string;
  kind: ScheduleKind;
  reviewItemId?: number;
  reviewKind?: string;
  allDaySpan: AllDaySpanPosition;
  excludable?: boolean;
  opensReviewQueue?: boolean;
}

export interface ScheduleDayMetadata {
  isWeekend: boolean;
}

export interface ScheduleGapOverlay {
  id: string;
  day: string;
  startMinutes: number;
  endMinutes: number;
  gapWindowStart: string;
  gapWindowEnd: string;
}

export type ScheduleItem = SchedulerItem<ScheduleMetadata>;
export type ScheduleDay = SchedulerDay<ScheduleDayMetadata>;
export type ScheduleChange = SchedulerChange<ScheduleMetadata>;
export type SchedulePlacement = Pick<
  ScheduleItem,
  "day" | "endMinutes" | "startMinutes"
>;
