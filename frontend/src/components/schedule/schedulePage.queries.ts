import {
  useAIConfigured,
  useCategories,
  useClassifyAIEndpoint,
  useCreateGapFill,
  useCreateManualEvent,
  useCurrentPeriod,
  useDeleteManualEvent,
  useEvents,
  useGapFills,
  useGapTimeline,
  useOpenReviewItems,
  usePeriods,
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
  const updateManualEventMutation = useUpdateManualEvent();
  const deleteManualEventMutation = useDeleteManualEvent();

  const aiConfig = useAIConfigured();
  const aiClassification = useClassifyAIEndpoint(aiConfig.baseURL);

  return {
    periodsQuery,
    currentPeriodQuery,
    categoriesQuery,
    createManualEventMutation,
    createGapFillMutation,
    suggestGapFillMutation,
    updateManualEventMutation,
    deleteManualEventMutation,
    aiConfig,
    aiClassification,
  };
}

export function useSchedulePagePeriodQueries(activePeriodId: number | undefined) {
  const eventsQuery = useEvents(activePeriodId);
  const gapFillsQuery = useGapFills(activePeriodId);
  const gapTimelineQuery = useGapTimeline(activePeriodId);
  const reviewItemsQuery = useOpenReviewItems(activePeriodId);
  const tzSegmentsQuery = useTzSegments(activePeriodId);

  return {
    eventsQuery,
    gapFillsQuery,
    gapTimelineQuery,
    reviewItemsQuery,
    tzSegmentsQuery,
  };
}
