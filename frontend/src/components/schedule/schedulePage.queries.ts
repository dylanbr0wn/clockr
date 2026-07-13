import {
  useAIConfigured,
  useCategories,
  useClassifyAIEndpoint,
  useCreateGapFill,
  useCreateManualEvent,
  useCurrentPeriod,
  useDeleteManualEvent,
  useExcludeEvent,
  useEventCategoryOverlays,
  useEvents,
  useGapFills,
  useGapTimeline,
  useReviewDecisions,
  usePeriods,
  useListGapEvidence,
  useSuggestGapFill,
  useTzSegments,
  useUpdateManualEvent,
} from "@/lib/api";

export function useSchedulePageBaseQueries(today: string, currentTimeZone: string) {
  const periodsQuery = usePeriods();
  const currentPeriodQuery = useCurrentPeriod(today, currentTimeZone);
  const categoriesQuery = useCategories();

  const createManualEventMutation = useCreateManualEvent();
  const createGapFillMutation = useCreateGapFill();
  const suggestGapFillMutation = useSuggestGapFill();
  const listGapEvidenceMutation = useListGapEvidence();
  const updateManualEventMutation = useUpdateManualEvent();
  const deleteManualEventMutation = useDeleteManualEvent();
  const excludeEventMutation = useExcludeEvent();

  const aiConfig = useAIConfigured();
  const aiClassification = useClassifyAIEndpoint(aiConfig.baseURL);

  return {
    periodsQuery,
    currentPeriodQuery,
    categoriesQuery,
    createManualEventMutation,
    createGapFillMutation,
    suggestGapFillMutation,
    listGapEvidenceMutation,
    updateManualEventMutation,
    deleteManualEventMutation,
    excludeEventMutation,
    aiConfig,
    aiClassification,
  };
}

export function useSchedulePagePeriodQueries(activePeriodId: number | undefined) {
  const eventsQuery = useEvents(activePeriodId);
  const eventCategoryOverlaysQuery = useEventCategoryOverlays(activePeriodId);
  const gapFillsQuery = useGapFills(activePeriodId);
  const gapTimelineQuery = useGapTimeline(activePeriodId);
  const reviewDecisionsQuery = useReviewDecisions(activePeriodId);
  const tzSegmentsQuery = useTzSegments(activePeriodId);

  return {
    eventsQuery,
    eventCategoryOverlaysQuery,
    gapFillsQuery,
    gapTimelineQuery,
    reviewDecisionsQuery,
    tzSegmentsQuery,
  };
}
