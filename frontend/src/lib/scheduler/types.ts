export type SchedulerInteraction = "move" | "resize-start" | "resize-end" | "create";

export interface SchedulerItem<TMetadata = unknown> {
  id: string;
  day: string;
  startMinutes: number;
  endMinutes: number;
  disabled?: boolean;
  metadata?: TMetadata;
}

export interface SchedulerDay<TMetadata = unknown> {
  date: string;
  label?: string;
  disabled?: boolean;
  metadata?: TMetadata;
}

export interface SchedulerVisibleRange {
  startMinutes: number;
  endMinutes: number;
}

export interface SchedulerConfig {
  slotMinutes: number;
  minDurationMinutes: number;
  createDurationMinutes: number;
  maxDays: number;
  /** Rendered vertical range. Defaults to the whole day so callers can scroll. */
  scheduleStartMinutes: number;
  scheduleEndMinutes: number;
  /** Normal working window, useful for app-level highlighting. */
  workingStartMinutes: number;
  workingEndMinutes: number;
  /** Pixels the pointer must travel before a drag starts. Keeps clicks from becoming drags. */
  dragThresholdPx: number;
  /** Allow dragging on empty column space to create a range. Click-to-create always works. */
  dragToCreate: boolean;
  /** Arrow-key move/resize on focused items. */
  keyboard: boolean;
}

export interface SchedulerCreateRequest {
  day: string;
  startMinutes: number;
  endMinutes: number;
}

export interface SchedulerChange<TMetadata = unknown> {
  itemId: string;
  day: string;
  startMinutes: number;
  endMinutes: number;
  interaction: SchedulerInteraction;
  item: SchedulerItem<TMetadata>;
}

export interface SchedulerLayoutItem<TMetadata = unknown> {
  item: SchedulerItem<TMetadata>;
  day: string;
  topPercent: number;
  heightPercent: number;
  leftPercent: number;
  widthPercent: number;
  lane: number;
  laneCount: number;
  overlaps: boolean;
  isPreview: boolean;
}

export interface SchedulerOptions<TItemMetadata = unknown, TDayMetadata = unknown> {
  days: SchedulerDay<TDayMetadata>[];
  items: SchedulerItem<TItemMetadata>[];
  config?: Partial<SchedulerConfig>;
  onCreate?: (request: SchedulerCreateRequest) => void;
  onPreviewChange?: (change: SchedulerChange<TItemMetadata>) => void;
  onCommitChange?: (change: SchedulerChange<TItemMetadata>) => void;
  /**
   * Constrain or reject a pending change before it is previewed or committed.
   * Return the change (possibly adjusted, e.g. snapped to neighbours) to accept it,
   * or null to reject it and keep the previous state.
   */
  transformChange?: (
    change: SchedulerChange<TItemMetadata>,
  ) => SchedulerChange<TItemMetadata> | null;
}

/** id of the synthetic item injected into layouts while drag-creating. */
export const CREATE_PREVIEW_ITEM_ID = "__scheduler-create-preview__";

export const DEFAULT_SCHEDULER_CONFIG: SchedulerConfig = {
  slotMinutes: 15,
  minDurationMinutes: 15,
  createDurationMinutes: 60,
  maxDays: 14,
  scheduleStartMinutes: 0,
  scheduleEndMinutes: 24 * 60,
  workingStartMinutes: 8 * 60,
  workingEndMinutes: 18 * 60,
  dragThresholdPx: 4,
  dragToCreate: true,
  keyboard: true,
};
