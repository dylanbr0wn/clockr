import { useCallback, useEffect, useMemo, useState, type SetStateAction } from "react";
import {
  timeEntryItemId,
  defaultTimeZone,
  errorMessage,
  localDateKey,
  type AllDayChip,
  type ScheduleGapOverlay,
} from "@/lib/schedule";
import { useJsonSetting } from "../settings/useJsonSetting";
import {
  DEFAULT_PRIVACY_FIELDS,
  formatPrivacySharingSummary,
} from "@/lib/ai/privacy";
import { useSchedulePageEditor } from "./schedulePage.editor";
import { useSchedulePageBaseQueries, useSchedulePagePeriodQueries } from "./schedulePage.queries";
import {
  buildSchedulePageDerived,
  resolveActivePeriod,
} from "./schedulePage.selectors";
import { buildSchedulePageStatus } from "./schedulePage.status";
import type {
  EditableScheduleEvent,
  ScheduleEventEditValues,
  SchedulePageViewModel,
  ScheduleViewDayCount,
} from "./schedulePage.types";
import {
  parseScheduleViewDayCount,
  SCHEDULE_VIEW_DAY_COUNT_KEY,
} from "./schedulePage.types";
import { useScheduleGapSuggest } from "./useScheduleGapSuggest";
import { mergePeriods } from "./useSchedulePage.helpers";

export type {
  EditableScheduleEvent,
  ScheduleEventEditValues,
  SchedulePageViewModel,
  ScheduleViewDayCount,
} from "./schedulePage.types";
export { SCHEDULE_VIEW_DAY_OPTIONS } from "./schedulePage.types";

export function useSchedulePage(): SchedulePageViewModel {
  const [selectedPeriodId, setSelectedPeriodId] = useState<number | null>(null);
  const viewDayCountSetting = useJsonSetting<number>(
    SCHEDULE_VIEW_DAY_COUNT_KEY,
    7,
  );
  const privacyFieldsSetting = useJsonSetting(
    "privacy.fields",
    DEFAULT_PRIVACY_FIELDS,
  );
  const viewDayCount = parseScheduleViewDayCount(viewDayCountSetting.value);
  const setViewDayCount = useCallback(
    (next: SetStateAction<ScheduleViewDayCount>) => {
      const resolved =
        typeof next === "function"
          ? parseScheduleViewDayCount(next(viewDayCount))
          : parseScheduleViewDayCount(next);
      viewDayCountSetting.setValue(resolved);
    },
    [viewDayCount, viewDayCountSetting],
  );
  const [reviewQueueOpen, setReviewQueueOpen] = useState(false);
  const [convertingChip, setConvertingChip] = useState<AllDayChip | null>(null);
  const today = useMemo(() => localDateKey(), []);
  const currentTimeZone = useMemo(() => defaultTimeZone(), []);
  const base = useSchedulePageBaseQueries(today, currentTimeZone);
  const persistedPeriods = base.periodsQuery.data ?? [];
  const currentPeriod = base.currentPeriodQuery.data ?? null;
  const categories = base.categoriesQuery.data ?? [];
  const projects = base.projectsQuery.data ?? [];
  const preloadedPeriods = useMemo(
    () => mergePeriods(persistedPeriods, currentPeriod),
    [currentPeriod, persistedPeriods],
  );
  const activePeriod = resolveActivePeriod({
    selectedPeriodId,
    currentPeriod,
    periods: preloadedPeriods,
    today,
  });
  const activePeriodId = activePeriod?.id;

  const period = useSchedulePagePeriodQueries(activePeriodId);
  const timeEntries = period.timeEntriesQuery.data ?? [];
  const editor = useSchedulePageEditor({
    activePeriodId,
    timeEntries,
    createTimeEntryMutation: base.createTimeEntryMutation,
    updateTimeEntryMutation: base.updateTimeEntryMutation,
    adjustDraftTimeEntryMutation: base.adjustDraftTimeEntryMutation,
    deleteTimeEntryMutation: base.deleteTimeEntryMutation,
    excludeEventMutation: base.excludeEventMutation,
  });

  const derived = useMemo(
    () =>
      buildSchedulePageDerived({
        selectedPeriodId,
        viewDayCount,
        today,
        persistedPeriods,
        currentPeriod,
        categories,
        events: period.eventsQuery.data ?? [],
        eventCategoryOverlays: period.eventCategoryOverlaysQuery.data ?? [],
        timeEntries,
        gapTimeline: period.gapTimelineQuery.data ?? [],
        reviewDecisions: period.reviewDecisionsQuery.data ?? [],
        tzSegments: period.tzSegmentsQuery.data ?? [],
        draftPlacements: editor.draftPlacements,
        pendingCreate: editor.pendingCreate,
        editingItemId: editor.editingItemId,
      }),
    [
      categories,
      currentPeriod,
      editor.draftPlacements,
      editor.editingItemId,
      editor.pendingCreate,
      period.eventsQuery.data,
      period.eventCategoryOverlaysQuery.data,
      timeEntries,
      period.gapTimelineQuery.data,
      period.reviewDecisionsQuery.data,
      period.tzSegmentsQuery.data,
      persistedPeriods,
      selectedPeriodId,
      today,
      viewDayCount,
    ],
  );

  const draftingEntry = useMemo(() => {
    if (!editor.draftingItemId) {
      return null;
    }
    return derived.timeEntriesByItemId.get(editor.draftingItemId) ?? null;
  }, [derived.timeEntriesByItemId, editor.draftingItemId]);

  const draftingPlacement = useMemo(() => {
    if (!editor.draftingItemId) {
      return null;
    }
    return editor.draftPlacements[editor.draftingItemId] ?? null;
  }, [editor.draftPlacements, editor.draftingItemId]);

  useEffect(() => {
    setSelectedPeriodId((current) => {
      if (
        currentPeriod &&
        (!current || !derived.periods.some((period) => period.id === current))
      ) {
        return currentPeriod.id;
      }

      if (current && derived.periods.some((period) => period.id === current)) {
        return current;
      }

      return currentPeriod?.id ?? derived.periods[0]?.id ?? null;
    });
  }, [currentPeriod, derived.periods]);

  const gapSuggest = useScheduleGapSuggest({
    activePeriodId: derived.activePeriodId,
    aiConfigured: base.aiConfig.isConfigured,
    suggestGapFillMutation: base.suggestGapFillMutation,
    listGapEvidenceMutation: base.listGapEvidenceMutation,
    createGapTimeEntryMutation: base.createGapTimeEntryMutation,
    resetKey: derived.activePeriodId,
  });

  useEffect(() => {
    editor.clearForPeriodChange();
    setReviewQueueOpen(false);
    setConvertingChip(null);
  }, [derived.activePeriodId]);

  const handleSelectGap = (overlay: ScheduleGapOverlay) => {
    editor.setPendingCreate(null);
    editor.setEditingItemId(null);
    editor.setDraftingItemId(null);
    gapSuggest.handleSelectGap(overlay);
  };

  const handleConvertAllDayChip = (chip: AllDayChip) => {
    if (!chip.convertible) {
      return;
    }
    editor.setEditingItemId(null);
    editor.setDraftingItemId(null);
    editor.setPendingCreate(null);
    setConvertingChip(chip);
  };

  const handleConvertAllDay = (values: {
    day: string;
    startMinutes: number;
    endMinutes: number;
    categoryId?: number;
    description: string;
  }) => {
    if (!convertingChip || !activePeriodId) {
      return;
    }
    base.convertAllDayEventMutation.mutate(
      {
        eventId: convertingChip.eventId,
        input: {
          periodId: activePeriodId,
          day: values.day,
          startMinutes: values.startMinutes,
          endMinutes: values.endMinutes,
          categoryId: values.categoryId,
          description: values.description,
          workType: "worked",
          billableStatus: "unset",
        },
      },
      {
        onSuccess: (entry) => {
          setConvertingChip(null);
          editor.setDraftingItemId(timeEntryItemId(entry.id));
        },
      },
    );
  };

  const handleConfirmDraft = (payload: {
    values?: {
      day: string;
      startMinutes: number;
      endMinutes: number;
      categoryId?: number;
      description: string;
      workType: string;
      projectId?: number;
      billableStatus: string;
    };
    overnightPolicy?: string;
    overlapResolution?: string;
  }) => {
    if (!draftingEntry) {
      return;
    }

    const confirm = () => {
      base.confirmTimeEntryMutation.mutate(
        {
          id: draftingEntry.id,
          periodId: draftingEntry.periodId,
          overnightPolicy: payload.overnightPolicy,
          overlapResolution: payload.overlapResolution,
        },
        { onSuccess: () => editor.setDraftingItemId(null) },
      );
    };

    if (!payload.values) {
      confirm();
      return;
    }

    const values = payload.values;
    base.adjustDraftTimeEntryMutation.mutate(
      {
        id: draftingEntry.id,
        periodId: draftingEntry.periodId,
        day: values.day,
        startMinutes: values.startMinutes,
        endMinutes: values.endMinutes,
        categoryId: values.categoryId,
        description: values.description,
        workType: values.workType,
        billableStatus: values.billableStatus,
        ...(typeof values.projectId === "number"
          ? { projectId: values.projectId }
          : {}),
      },
      { onSuccess: confirm },
    );
  };

  const handleRejectDraft = () => {
    if (!draftingEntry) {
      return;
    }
    base.rejectTimeEntryMutation.mutate(
      { id: draftingEntry.id, periodId: draftingEntry.periodId },
      { onSuccess: () => editor.setDraftingItemId(null) },
    );
  };

  const handleSplitDraft = (cutPoints: string[]) => {
    if (!draftingEntry) {
      return;
    }
    base.splitTimeEntryMutation.mutate(
      {
        id: draftingEntry.id,
        periodId: draftingEntry.periodId,
        cutPoints,
      },
      {
        onSuccess: (entries) => {
          const first = entries[0];
          editor.setDraftingItemId(first ? timeEntryItemId(first.id) : null);
        },
      },
    );
  };

  const aiLocal = base.aiClassification.data?.local ?? false;
  const aiPrivacyLabel = aiLocal
    ? "Private · on-device"
    : `Cloud · sharing ${formatPrivacySharingSummary(privacyFieldsSetting.value)}`;

  const draftEditorPending =
    base.adjustDraftTimeEntryMutation.isPending ||
    base.confirmTimeEntryMutation.isPending ||
    base.rejectTimeEntryMutation.isPending ||
    base.splitTimeEntryMutation.isPending;

  const draftMutationError =
    base.adjustDraftTimeEntryMutation.error ??
    base.confirmTimeEntryMutation.error ??
    base.rejectTimeEntryMutation.error ??
    base.splitTimeEntryMutation.error ??
    null;
  const draftEditorError = draftMutationError
    ? errorMessage(draftMutationError)
    : null;

  const status = buildSchedulePageStatus({
    loadingFlags: [
      base.periodsQuery.isLoading,
      base.currentPeriodQuery.isLoading,
      base.categoriesQuery.isLoading,
      base.projectsQuery.isLoading,
      period.eventsQuery.isLoading,
      period.timeEntriesQuery.isLoading,
      period.gapTimelineQuery.isLoading,
      period.reviewDecisionsQuery.isLoading,
      period.tzSegmentsQuery.isLoading,
      base.createTimeEntryMutation.isPending,
      base.createGapTimeEntryMutation.isPending,
      gapSuggest.gapSuggestPending,
      base.updateTimeEntryMutation.isPending,
      base.adjustDraftTimeEntryMutation.isPending,
      base.confirmTimeEntryMutation.isPending,
      base.rejectTimeEntryMutation.isPending,
      base.splitTimeEntryMutation.isPending,
      base.convertAllDayEventMutation.isPending,
      base.deleteTimeEntryMutation.isPending,
      base.excludeEventMutation.isPending,
    ],
    errors: [
      base.periodsQuery.error,
      base.currentPeriodQuery.error,
      base.categoriesQuery.error,
      base.projectsQuery.error,
      period.eventsQuery.error,
      period.timeEntriesQuery.error,
      period.gapTimelineQuery.error,
      period.reviewDecisionsQuery.error,
      period.tzSegmentsQuery.error,
      base.createTimeEntryMutation.error,
      base.createGapTimeEntryMutation.error,
      gapSuggest.gapSuggestError,
      base.updateTimeEntryMutation.error,
      base.adjustDraftTimeEntryMutation.error,
      base.confirmTimeEntryMutation.error,
      base.rejectTimeEntryMutation.error,
      base.splitTimeEntryMutation.error,
      base.convertAllDayEventMutation.error,
      base.deleteTimeEntryMutation.error,
      base.excludeEventMutation.error,
    ],
    eventsCount: period.eventsQuery.data?.length ?? 0,
    timeEntriesCount: timeEntries.length,
    categoriesCount: categories.length,
    reviewDecisionsCount: period.reviewDecisionsQuery.data?.length ?? 0,
  });

  return {
    selectedPeriodId,
    setSelectedPeriodId,
    viewDayCount,
    setViewDayCount,
    periods: derived.periods,
    activePeriod: derived.activePeriod,
    activePeriodId: derived.activePeriodId,
    categories,
    projects,
    events: period.eventsQuery.data ?? [],
    timeEntries,
    tzSegments: period.tzSegmentsQuery.data ?? [],
    reviewDecisions: period.reviewDecisionsQuery.data ?? [],
    days: derived.days,
    items: derived.items,
    allDayChipsByDay: derived.allDayChipsByDay,
    visibleGaps: derived.visibleGaps,
    resettableDays: derived.resettableDays,
    totals: derived.totals,
    visibleDayCount: derived.visibleDayCount,
    preview: editor.preview,
    setPreview: editor.setPreview,
    isBackendLoading: status.isBackendLoading,
    backendError: status.backendError,
    counts: status.counts,
    createPending: base.createTimeEntryMutation.isPending,
    editingEvent: derived.editingEvent as EditableScheduleEvent | null,
    editEventPending: base.updateTimeEntryMutation.isPending,
    handleCreate: editor.handleCreate,
    handleCommit: editor.handleCommit,
    handleOpenEventEditor: editor.handleOpenEventEditor,
    handleDuplicateEvent: editor.handleDuplicateEvent,
    handleRemoveEvent: editor.handleRemoveEvent,
    handleExcludeEvent: editor.handleExcludeEvent,
    handleExcludeAllDayChip: editor.handleExcludeAllDayChip,
    handleConvertAllDayChip,
    handleResetDay: editor.handleResetDay,
    handleCloseEventEditor: editor.handleCloseEventEditor,
    handleSaveEventEdit: editor.handleSaveEventEdit as (
      values: ScheduleEventEditValues,
    ) => void,
    draftingEntry,
    draftingPlacement,
    draftEditorPending,
    draftEditorError,
    handleOpenDraftEditor: editor.handleOpenDraftEditor,
    handleCloseDraftEditor: editor.handleCloseDraftEditor,
    handleAdjustDraft: editor.handleAdjustDraft as (
      values: ScheduleEventEditValues,
    ) => void,
    handleConfirmDraft,
    handleRejectDraft,
    handleSplitDraft,
    convertingChip,
    convertAllDayPending: base.convertAllDayEventMutation.isPending,
    handleCloseConvertAllDay: () => setConvertingChip(null),
    handleConvertAllDay,
    reviewQueueOpen,
    setReviewQueueOpen,
    selectedGap: gapSuggest.selectedGap,
    gapSuggestion: gapSuggest.gapSuggestion,
    gapEvidenceItems: gapSuggest.gapEvidenceItems,
    gapSuggestOpen: gapSuggest.gapSuggestOpen,
    gapSuggestPending: gapSuggest.gapSuggestPending,
    gapEvidencePending: gapSuggest.gapEvidencePending,
    gapSuggestSaving: gapSuggest.gapSuggestSaving,
    gapSuggestError: gapSuggest.gapSuggestError,
    gapEvidenceError: gapSuggest.gapEvidenceError,
    aiConfigured: base.aiConfig.isConfigured,
    aiLocal,
    aiPrivacyLabel,
    handleSelectGap,
    handleCloseGapSuggest: gapSuggest.handleCloseGapSuggest,
    handleRetryGapSuggest: gapSuggest.handleRetryGapSuggest,
    handleConfirmGapSuggest: gapSuggest.handleConfirmGapSuggest,
  };
}
