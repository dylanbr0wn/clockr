import { useEffect, useMemo, useState } from "react";
import { defaultTimeZone, localDateKey, type ScheduleGapOverlay } from "@/lib/schedule";
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
  const [viewDayCount, setViewDayCount] = useState<ScheduleViewDayCount>(7);
  const [reviewQueueOpen, setReviewQueueOpen] = useState(false);
  const today = useMemo(() => localDateKey(), []);
  const currentTimeZone = useMemo(() => defaultTimeZone(), []);
  const base = useSchedulePageBaseQueries(today, currentTimeZone);
  const persistedPeriods = base.periodsQuery.data ?? [];
  const currentPeriod = base.currentPeriodQuery.data ?? null;
  const categories = base.categoriesQuery.data ?? [];
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
  const editor = useSchedulePageEditor({
    activePeriodId,
    gapFills: period.gapFillsQuery.data ?? [],
    createManualEventMutation: base.createManualEventMutation,
    updateManualEventMutation: base.updateManualEventMutation,
    deleteManualEventMutation: base.deleteManualEventMutation,
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
        gapFills: period.gapFillsQuery.data ?? [],
        gapTimeline: period.gapTimelineQuery.data ?? [],
        reviewItems: period.reviewItemsQuery.data ?? [],
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
      period.gapFillsQuery.data,
      period.gapTimelineQuery.data,
      period.reviewItemsQuery.data,
      period.tzSegmentsQuery.data,
      persistedPeriods,
      selectedPeriodId,
      today,
      viewDayCount,
    ],
  );

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
    createGapFillMutation: base.createGapFillMutation,
    resetKey: derived.activePeriodId,
  });

  useEffect(() => {
    editor.clearForPeriodChange();
    setReviewQueueOpen(false);
  }, [derived.activePeriodId]);

  const handleSelectGap = (overlay: ScheduleGapOverlay) => {
    editor.setPendingCreate(null);
    editor.setEditingItemId(null);
    gapSuggest.handleSelectGap(overlay);
  };

  const status = buildSchedulePageStatus({
    loadingFlags: [
      base.periodsQuery.isLoading,
      base.currentPeriodQuery.isLoading,
      base.categoriesQuery.isLoading,
      period.eventsQuery.isLoading,
      period.gapFillsQuery.isLoading,
      period.gapTimelineQuery.isLoading,
      period.reviewItemsQuery.isLoading,
      period.tzSegmentsQuery.isLoading,
      base.createManualEventMutation.isPending,
      base.createGapFillMutation.isPending,
      gapSuggest.gapSuggestPending,
      base.updateManualEventMutation.isPending,
      base.deleteManualEventMutation.isPending,
    ],
    errors: [
      base.periodsQuery.error,
      base.currentPeriodQuery.error,
      base.categoriesQuery.error,
      period.eventsQuery.error,
      period.gapFillsQuery.error,
      period.gapTimelineQuery.error,
      period.reviewItemsQuery.error,
      period.tzSegmentsQuery.error,
      base.createManualEventMutation.error,
      base.createGapFillMutation.error,
      gapSuggest.gapSuggestError,
      base.updateManualEventMutation.error,
      base.deleteManualEventMutation.error,
    ],
    eventsCount: period.eventsQuery.data?.length ?? 0,
    gapFillsCount: period.gapFillsQuery.data?.length ?? 0,
    categoriesCount: categories.length,
    reviewItemsCount: period.reviewItemsQuery.data?.length ?? 0,
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
    createPending: base.createManualEventMutation.isPending,
    editingEvent: derived.editingEvent as EditableScheduleEvent | null,
    editEventPending: base.updateManualEventMutation.isPending,
    handleCreate: editor.handleCreate,
    handleCommit: editor.handleCommit,
    handleOpenEventEditor: editor.handleOpenEventEditor,
    handleDuplicateEvent: editor.handleDuplicateEvent,
    handleRemoveEvent: editor.handleRemoveEvent,
    handleResetDay: editor.handleResetDay,
    handleCloseEventEditor: editor.handleCloseEventEditor,
    handleSaveEventEdit: editor.handleSaveEventEdit as (
      values: ScheduleEventEditValues,
    ) => void,
    reviewQueueOpen,
    setReviewQueueOpen,
    selectedGap: gapSuggest.selectedGap,
    gapSuggestion: gapSuggest.gapSuggestion,
    gapSuggestOpen: gapSuggest.gapSuggestOpen,
    gapSuggestPending: gapSuggest.gapSuggestPending,
    gapSuggestSaving: gapSuggest.gapSuggestSaving,
    gapSuggestError: gapSuggest.gapSuggestError,
    aiConfigured: base.aiConfig.isConfigured,
    aiLocal: base.aiClassification.data?.local ?? false,
    handleSelectGap,
    handleCloseGapSuggest: gapSuggest.handleCloseGapSuggest,
    handleRetryGapSuggest: gapSuggest.handleRetryGapSuggest,
    handleConfirmGapSuggest: gapSuggest.handleConfirmGapSuggest,
  };
}
