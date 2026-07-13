import {
  useAIConfigured,
  useCategories,
  useClassifyAIEndpoint,
  useCreateGapTimeEntry,
  useCreateTimeEntry,
  useCurrentPeriod,
  useDeleteTimeEntry,
  useExcludeEvent,
  useEventCategoryOverlays,
  useEvents,
  useTimeEntries,
  useGapTimeline,
  useReviewDecisions,
  usePeriods,
  useListGapEvidence,
  useSuggestGapFill,
  useTzSegments,
  useUpdateTimeEntry,
} from "@/lib/api";

export function useSchedulePageBaseQueries(today: string, currentTimeZone: string) {
  const periodsQuery = usePeriods();
  const currentPeriodQuery = useCurrentPeriod(today, currentTimeZone);
  const categoriesQuery = useCategories(true);

  const createTimeEntryMutation = useCreateTimeEntry();
  const createGapTimeEntryMutation = useCreateGapTimeEntry();
  const suggestGapFillMutation = useSuggestGapFill();
  const listGapEvidenceMutation = useListGapEvidence();
  const updateTimeEntryMutation = useUpdateTimeEntry();
  const deleteTimeEntryMutation = useDeleteTimeEntry();
  const excludeEventMutation = useExcludeEvent();

  const aiConfig = useAIConfigured();
  const aiClassification = useClassifyAIEndpoint(aiConfig.baseURL);

  return {
    periodsQuery,
    currentPeriodQuery,
    categoriesQuery,
    createTimeEntryMutation,
    createGapTimeEntryMutation,
    suggestGapFillMutation,
    listGapEvidenceMutation,
    updateTimeEntryMutation,
    deleteTimeEntryMutation,
    excludeEventMutation,
    aiConfig,
    aiClassification,
  };
}

export function useSchedulePagePeriodQueries(activePeriodId: number | undefined) {
  const eventsQuery = useEvents(activePeriodId);
  const eventCategoryOverlaysQuery = useEventCategoryOverlays(activePeriodId);
  const timeEntriesQuery = useTimeEntries(activePeriodId);
  const gapTimelineQuery = useGapTimeline(activePeriodId);
  const reviewDecisionsQuery = useReviewDecisions(activePeriodId);
  const tzSegmentsQuery = useTzSegments(activePeriodId);

  return {
    eventsQuery,
    eventCategoryOverlaysQuery,
    timeEntriesQuery,
    gapTimelineQuery,
    reviewDecisionsQuery,
    tzSegmentsQuery,
  };
}
