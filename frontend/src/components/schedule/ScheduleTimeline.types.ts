import type { SchedulerCreateRequest } from "@/lib/scheduler";
import type {
  AllDayChip,
  ScheduleChange,
  ScheduleDay,
  ScheduleGapOverlay,
  ScheduleItem,
} from "@/lib/schedule";

export interface ScheduleTimelineData {
  days: ScheduleDay[];
  items: ScheduleItem[];
  allDayChipsByDay: Map<string, AllDayChip[]>;
  visibleGaps: ScheduleGapOverlay[];
  resettableDays: ReadonlySet<string>;
  visibleDayCount: number;
  aiConfigured: boolean;
}

export interface ScheduleTimelineActions {
  onCreate: (request: SchedulerCreateRequest) => void;
  onPreviewChange: (change: ScheduleChange | null) => void;
  onCommitChange: (change: ScheduleChange) => void;
  onEditItem: (item: ScheduleItem) => void;
  onDuplicateItem: (item: ScheduleItem) => void;
  onRemoveItem: (item: ScheduleItem) => void;
  onExcludeItem: (item: ScheduleItem) => void;
  onExcludeAllDayChip: (chip: AllDayChip) => void;
  onResetDay: (day: string) => void;
  onSelectGap: (gap: ScheduleGapOverlay) => void;
  onOpenReviewQueue: () => void;
}
