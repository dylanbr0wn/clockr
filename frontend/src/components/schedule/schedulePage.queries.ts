import {
  useAIConfigured,
  useAdjustDraftTimeEntry,
  useCategories,
  useClassifyAIEndpoint,
  useConfirmTimeEntry,
  useConvertAllDayEvent,
  useCreateGapTimeEntry,
  useCreateTimeEntry,
  useCurrentPeriod,
  useDeleteTimeEntry,
  useExcludeEvent,
  useEventCategoryOverlays,
  useEvents,
  useTimeEntries,
  useGapTimeline,
  useProjects,
  useRejectTimeEntry,
  useReviewDecisions,
  usePeriods,
  useListGapEvidence,
  useSplitTimeEntry,
  useSuggestGapFill,
  useTzSegments,
  useUpdateTimeEntry,
} from "@/lib/api";

export function useSchedulePageBaseQueries(today: string, currentTimeZone: string) {
  const periodsQuery = usePeriods();
  const currentPeriodQuery = useCurrentPeriod(today, currentTimeZone);
  const categoriesQuery = useCategories(true);
  const projectsQuery = useProjects(true);

  const createTimeEntryMutation = useCreateTimeEntry();
  const createGapTimeEntryMutation = useCreateGapTimeEntry();
  const suggestGapFillMutation = useSuggestGapFill();
  const listGapEvidenceMutation = useListGapEvidence();
  const updateTimeEntryMutation = useUpdateTimeEntry();
  const adjustDraftTimeEntryMutation = useAdjustDraftTimeEntry();
  const confirmTimeEntryMutation = useConfirmTimeEntry();
  const rejectTimeEntryMutation = useRejectTimeEntry();
  const splitTimeEntryMutation = useSplitTimeEntry();
  const convertAllDayEventMutation = useConvertAllDayEvent();
  const deleteTimeEntryMutation = useDeleteTimeEntry();
  const excludeEventMutation = useExcludeEvent();

  const aiConfig = useAIConfigured();
  const aiClassification = useClassifyAIEndpoint(aiConfig.baseURL);

  return {
    periodsQuery,
    currentPeriodQuery,
    categoriesQuery,
    projectsQuery,
    createTimeEntryMutation,
    createGapTimeEntryMutation,
    suggestGapFillMutation,
    listGapEvidenceMutation,
    updateTimeEntryMutation,
    adjustDraftTimeEntryMutation,
    confirmTimeEntryMutation,
    rejectTimeEntryMutation,
    splitTimeEntryMutation,
    convertAllDayEventMutation,
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
