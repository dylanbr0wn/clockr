import type {
  Category,
  DayTimeline,
  Event,
  GapEvidenceItem,
  GapSuggestion,
  TimeEntry,
  Period,
  Project,
  ReviewDecision,
  TzSegment,
} from "@/lib/api";
import type { SchedulerCreateRequest } from "@/lib/scheduler";
import type {
  AllDayChip,
  ScheduleChange,
  ScheduleDay,
  ScheduleGapOverlay,
  ScheduleItem,
  SchedulePlacement,
} from "@/lib/schedule";
import type { Dispatch, SetStateAction } from "react";
import type { SelectedGap } from "./GapSuggestDialog";

export type ScheduleViewDayCount = 1 | 7 | 14;

export const SCHEDULE_VIEW_DAY_OPTIONS: ScheduleViewDayCount[] = [1, 7, 14];

export const SCHEDULE_VIEW_DAY_COUNT_KEY = "schedule.viewDayCount";

export function parseScheduleViewDayCount(raw: unknown): ScheduleViewDayCount {
  if (
    typeof raw === "number" &&
    SCHEDULE_VIEW_DAY_OPTIONS.includes(raw as ScheduleViewDayCount)
  ) {
    return raw as ScheduleViewDayCount;
  }

  return 7;
}

export interface EditableScheduleEvent {
  id: string;
  timeEntryId?: number;
  periodId: number;
  day: string;
  startMinutes: number;
  endMinutes: number;
  category: string;
  categoryId?: number;
  note: string;
  description: string;
  workType: string;
  projectId?: number;
  billableStatus: string;
  isNew?: boolean;
}

export interface ScheduleEventEditValues {
  day: string;
  startMinutes: number;
  endMinutes: number;
  categoryId?: number;
  note: string;
  description: string;
  workType: string;
  projectId?: number;
  billableStatus: string;
}

export interface SchedulePageViewModel {
  selectedPeriodId: number | null;
  setSelectedPeriodId: Dispatch<SetStateAction<number | null>>;
  viewDayCount: ScheduleViewDayCount;
  setViewDayCount: Dispatch<SetStateAction<ScheduleViewDayCount>>;
  periods: Period[];
  activePeriod: Period | null;
  activePeriodId: number | undefined;
  categories: Category[];
  projects: Project[];
  events: Event[];
  reviewDecisions: ReviewDecision[];
  days: ScheduleDay[];
  items: ScheduleItem[];
  allDayChipsByDay: Map<string, AllDayChip[]>;
  visibleGaps: ScheduleGapOverlay[];
  resettableDays: ReadonlySet<string>;
  totals: Record<string, number>;
  visibleDayCount: number;
  preview: ScheduleChange | null;
  setPreview: Dispatch<SetStateAction<ScheduleChange | null>>;
  isBackendLoading: boolean;
  backendError: unknown;
  counts: {
    events: number;
    timeEntries: number;
    categories: number;
    reviewDecisions: number;
  };
  createPending: boolean;
  editingEvent: EditableScheduleEvent | null;
  editEventPending: boolean;
  handleCreate: (request: SchedulerCreateRequest) => void;
  handleCommit: (change: ScheduleChange) => void;
  handleOpenEventEditor: (item: ScheduleItem) => void;
  handleDuplicateEvent: (item: ScheduleItem) => void;
  handleRemoveEvent: (item: ScheduleItem) => void;
  handleExcludeEvent: (item: ScheduleItem) => void;
  handleExcludeAllDayChip: (chip: AllDayChip) => void;
  handleResetDay: (day: string) => void;
  handleCloseEventEditor: () => void;
  handleSaveEventEdit: (values: ScheduleEventEditValues) => void;
  reviewQueueOpen: boolean;
  setReviewQueueOpen: Dispatch<SetStateAction<boolean>>;
  selectedGap: SelectedGap | null;
  gapSuggestion: GapSuggestion | null;
  gapEvidenceItems: GapEvidenceItem[];
  gapSuggestOpen: boolean;
  gapSuggestPending: boolean;
  gapEvidencePending: boolean;
  gapSuggestSaving: boolean;
  gapSuggestError: unknown;
  gapEvidenceError: unknown;
  aiConfigured: boolean;
  aiLocal: boolean;
  aiPrivacyLabel: string;
  handleSelectGap: (gap: ScheduleGapOverlay) => void;
  handleCloseGapSuggest: () => void;
  handleRetryGapSuggest: () => void;
  handleConfirmGapSuggest: (values: {
    categoryId?: number;
    description: string;
    workType: string;
    projectId?: number;
    billableStatus: string;
  }) => void;
}

export interface SchedulePageDataInputs {
  periods: Period[];
  currentPeriod: Period | null;
  categories: Category[];
  events: Event[];
  timeEntries: TimeEntry[];
  gapTimeline: DayTimeline[];
  reviewDecisions: ReviewDecision[];
  tzSegments: TzSegment[];
}

export interface SchedulePageEditorState {
  preview: ScheduleChange | null;
  setPreview: Dispatch<SetStateAction<ScheduleChange | null>>;
  editingItemId: string | null;
  pendingCreate: SchedulerCreateRequest | null;
  draftPlacements: Record<string, SchedulePlacement>;
}
