import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  classifyAIEndpoint,
  computeGaps,
  connectGoogle,
  createGapFill,
  createCategory,
  createManualEvent,
  deleteCategory,
  deleteManualEvent,
  disconnectGoogle,
  discoverLocalAIEndpoints,
  ensureCurrentPeriod,
  getSetting,
  listAIModels,
  listCalendars,
  listCategories,
  listEventCategoryOverlays,
  listEvents,
  listGapFills,
  listIntegrationConnections,
  listOpenReviewItems,
  listPeriods,
  listSelectedCalendars,
  listTzSegments,
  resolveReviewItem,
  saveAIConfig,
  saveAIEndpoint,
  saveAIModel,
  setCalendarDefaultCategory,
  setCalendarSelected,
  setSetting,
  suggestGapFill,
  syncPeriod,
  updateCategory,
  updateManualEvent,
  validateAIConfig,
} from "./shietService";
import type { TimeWindow } from "./types";

function parseJsonSetting<T>(raw: string | null | undefined, fallback: T): T {
  if (!raw) {
    return fallback;
  }

  try {
    return JSON.parse(raw) as T;
  } catch {
    return fallback;
  }
}

export const shietQueryKeys = {
  all: ["shiet"] as const,
  calendars: () => [...shietQueryKeys.all, "calendars"] as const,
  categories: () => [...shietQueryKeys.all, "categories"] as const,
  gapTimeline: (periodId: number) =>
    [...shietQueryKeys.period(periodId), "gapTimeline"] as const,
  period: (periodId: number) =>
    [...shietQueryKeys.periods(), periodId] as const,
  currentPeriod: (today: string, ianaTz: string) =>
    [...shietQueryKeys.periods(), "current", today, ianaTz] as const,
  periods: () => [...shietQueryKeys.all, "periods"] as const,
  periodEvents: (periodId: number) =>
    [...shietQueryKeys.period(periodId), "events"] as const,
  periodEventCategoryOverlays: (periodId: number) =>
    [...shietQueryKeys.period(periodId), "eventCategoryOverlays"] as const,
  periodGapFills: (periodId: number) =>
    [...shietQueryKeys.period(periodId), "gapFills"] as const,
  periodReviewItems: (periodId: number) =>
    [...shietQueryKeys.period(periodId), "reviewItems"] as const,
  periodTzSegments: (periodId: number) =>
    [...shietQueryKeys.period(periodId), "tzSegments"] as const,
  selectedCalendars: () =>
    [...shietQueryKeys.calendars(), "selected"] as const,
  connections: () => [...shietQueryKeys.all, "connections"] as const,
  setting: (key: string) => [...shietQueryKeys.all, "settings", key] as const,
  aiDiscovery: () => [...shietQueryKeys.all, "ai", "discovery"] as const,
  aiClassification: (baseURL: string) =>
    [...shietQueryKeys.all, "ai", "classification", baseURL] as const,
  aiModels: (baseURL: string) =>
    [...shietQueryKeys.all, "ai", "models", baseURL] as const,
  aiValidation: (baseURL: string, apiKey: string, model: string) =>
    [...shietQueryKeys.all, "ai", "validation", baseURL, apiKey, model] as const,
};

export function usePeriods() {
  return useQuery({
    queryKey: shietQueryKeys.periods(),
    queryFn: listPeriods,
  });
}

export function useCurrentPeriod(today: string, ianaTz: string) {
  return useQuery({
    queryKey: shietQueryKeys.currentPeriod(today, ianaTz),
    queryFn: () => ensureCurrentPeriod(today, ianaTz),
  });
}

export function useCategories() {
  return useQuery({
    queryKey: shietQueryKeys.categories(),
    queryFn: listCategories,
  });
}

function invalidateCategoryQueries(queryClient: ReturnType<typeof useQueryClient>) {
  void queryClient.invalidateQueries({
    queryKey: shietQueryKeys.categories(),
  });
  void queryClient.invalidateQueries({
    queryKey: shietQueryKeys.calendars(),
  });
  void queryClient.invalidateQueries({
    queryKey: shietQueryKeys.selectedCalendars(),
  });
}

export function useCreateCategory() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: createCategory,
    onSuccess: () => {
      invalidateCategoryQueries(queryClient);
    },
  });
}

export function useUpdateCategory() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: updateCategory,
    onSuccess: () => {
      invalidateCategoryQueries(queryClient);
    },
  });
}

export function useDeleteCategory() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: deleteCategory,
    onSuccess: () => {
      invalidateCategoryQueries(queryClient);
    },
  });
}

export function useEventCategoryOverlays(periodId: number | null | undefined) {
  return useQuery({
    enabled: typeof periodId === "number",
    queryKey: shietQueryKeys.periodEventCategoryOverlays(periodId ?? 0),
    queryFn: () => listEventCategoryOverlays(periodId as number),
  });
}

export function useCalendars() {
  return useQuery({
    queryKey: shietQueryKeys.calendars(),
    queryFn: listCalendars,
  });
}

export function useSelectedCalendars() {
  return useQuery({
    queryKey: shietQueryKeys.selectedCalendars(),
    queryFn: listSelectedCalendars,
  });
}

export function useEvents(periodId: number | null | undefined) {
  return useQuery({
    enabled: typeof periodId === "number",
    queryKey: shietQueryKeys.periodEvents(periodId ?? 0),
    queryFn: () => listEvents(periodId as number),
  });
}

export function useGapFills(periodId: number | null | undefined) {
  return useQuery({
    enabled: typeof periodId === "number",
    queryKey: shietQueryKeys.periodGapFills(periodId ?? 0),
    queryFn: () => listGapFills(periodId as number),
  });
}

export function useCreateManualEvent() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: createManualEvent,
    onSuccess: (gapFill, input) => {
      const periodId = gapFill.periodId || input.periodId;

      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.periodGapFills(periodId),
      });
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.gapTimeline(periodId),
      });
    },
  });
}

export function useCreateGapFill() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: createGapFill,
    onSuccess: (gapFill, input) => {
      const periodId = gapFill.periodId || input.periodId;

      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.periodGapFills(periodId),
      });
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.gapTimeline(periodId),
      });
    },
  });
}

export function useSuggestGapFill() {
  return useMutation({
    mutationFn: (window: TimeWindow) => suggestGapFill(window),
  });
}

export function useAIConfigured() {
  const baseURLSetting = useSetting("ai.base_url");
  const modelSetting = useSetting("ai.model");

  const baseURL = parseJsonSetting(baseURLSetting.data, "");
  const model = parseJsonSetting(modelSetting.data, "");

  return {
    isConfigured: Boolean(baseURL.trim() && model.trim()),
    baseURL,
    isLoading: baseURLSetting.isLoading || modelSetting.isLoading,
  };
}

export function useUpdateManualEvent() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: updateManualEvent,
    onSuccess: (gapFill, input) => {
      const periodId = gapFill.periodId || input.periodId;

      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.periodGapFills(periodId),
      });
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.gapTimeline(periodId),
      });
    },
  });
}

export function useDeleteManualEvent() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: deleteManualEvent,
    onSuccess: (result, input) => {
      const periodId = result.periodId || input.periodId;

      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.periodGapFills(periodId),
      });
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.gapTimeline(periodId),
      });
    },
  });
}

export function useOpenReviewItems(periodId: number | null | undefined) {
  return useQuery({
    enabled: typeof periodId === "number",
    queryKey: shietQueryKeys.periodReviewItems(periodId ?? 0),
    queryFn: () => listOpenReviewItems(periodId as number),
  });
}

export function useResolveReviewItem() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: resolveReviewItem,
    onSuccess: (result) => {
      const periodId = result.periodId;
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.periodReviewItems(periodId),
      });
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.periodEvents(periodId),
      });
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.periodGapFills(periodId),
      });
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.gapTimeline(periodId),
      });
    },
  });
}

export function useTzSegments(periodId: number | null | undefined) {
  return useQuery({
    enabled: typeof periodId === "number",
    queryKey: shietQueryKeys.periodTzSegments(periodId ?? 0),
    queryFn: () => listTzSegments(periodId as number),
  });
}

export function useGapTimeline(periodId: number | null | undefined) {
  return useQuery({
    enabled: typeof periodId === "number",
    queryKey: shietQueryKeys.gapTimeline(periodId ?? 0),
    queryFn: () => computeGaps(periodId as number),
  });
}

export function useSetting(key: string) {
  return useQuery({
    queryKey: shietQueryKeys.setting(key),
    queryFn: () => getSetting(key),
  });
}

export function useSetSetting() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ key, value }: { key: string; value: string }) =>
      setSetting(key, value),
    onMutate: async ({ key, value }) => {
      const queryKey = shietQueryKeys.setting(key);
      await queryClient.cancelQueries({ queryKey });
      const previous = queryClient.getQueryData<string | null | undefined>(
        queryKey,
      );
      queryClient.setQueryData(queryKey, value);
      return { key, previous };
    },
    onError: (_error, _variables, context) => {
      if (!context) {
        return;
      }
      queryClient.setQueryData(
        shietQueryKeys.setting(context.key),
        context.previous,
      );
    },
    onSettled: (_result, _error, { key }) => {
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.setting(key),
      });
    },
  });
}

export function useDiscoverLocalAIEndpoints() {
  return useQuery({
    queryKey: shietQueryKeys.aiDiscovery(),
    queryFn: discoverLocalAIEndpoints,
  });
}

export function useClassifyAIEndpoint(baseURL: string) {
  return useQuery({
    enabled: baseURL.trim().length > 0,
    queryKey: shietQueryKeys.aiClassification(baseURL),
    queryFn: () => classifyAIEndpoint(baseURL),
  });
}

export function useAIModels(baseURL: string, apiKey: string) {
  return useQuery({
    enabled: baseURL.trim().length > 0,
    queryKey: shietQueryKeys.aiModels(baseURL),
    queryFn: () => listAIModels(baseURL, apiKey),
    retry: false,
  });
}

export function useValidateAIConfig(
  baseURL: string,
  apiKey: string,
  model: string,
) {
  return useQuery({
    enabled: false,
    queryKey: shietQueryKeys.aiValidation(baseURL, apiKey, model),
    queryFn: () => validateAIConfig(baseURL, apiKey, model),
    retry: false,
  });
}

export function useClearAIModel() {
  const queryClient = useQueryClient();
  const queryKey = shietQueryKeys.setting("ai.model");

  return useMutation({
    mutationFn: () => setSetting("ai.model", '""'),
    onMutate: async () => {
      await queryClient.cancelQueries({ queryKey });
      const previous = queryClient.getQueryData<string | null | undefined>(
        queryKey,
      );
      queryClient.setQueryData(queryKey, '""');
      return { previous };
    },
    onError: (_error, _variables, context) => {
      if (context) {
        queryClient.setQueryData(queryKey, context.previous);
      }
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey });
    },
  });
}

export function useSaveAIEndpoint() {
  const queryClient = useQueryClient();
  const queryKey = shietQueryKeys.setting("ai.base_url");

  return useMutation({
    mutationFn: (baseURL: string) => saveAIEndpoint(baseURL),
    onMutate: async (baseURL) => {
      await queryClient.cancelQueries({ queryKey });
      const previous = queryClient.getQueryData<string | null | undefined>(
        queryKey,
      );
      queryClient.setQueryData(queryKey, JSON.stringify(baseURL));
      return { previous };
    },
    onError: (_error, _variables, context) => {
      if (context) {
        queryClient.setQueryData(queryKey, context.previous);
      }
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey });
    },
  });
}

export function useSaveAIModel() {
  const queryClient = useQueryClient();
  const queryKey = shietQueryKeys.setting("ai.model");

  return useMutation({
    mutationFn: (model: string) => saveAIModel(model),
    onMutate: async (model) => {
      await queryClient.cancelQueries({ queryKey });
      const previous = queryClient.getQueryData<string | null | undefined>(
        queryKey,
      );
      queryClient.setQueryData(queryKey, JSON.stringify(model));
      return { previous };
    },
    onError: (_error, _variables, context) => {
      if (context) {
        queryClient.setQueryData(queryKey, context.previous);
      }
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey });
    },
  });
}

export function useSaveAIConfig() {
  const queryClient = useQueryClient();
  const baseURLKey = shietQueryKeys.setting("ai.base_url");
  const modelKey = shietQueryKeys.setting("ai.model");

  return useMutation({
    mutationFn: ({
      baseURL,
      model,
    }: {
      baseURL: string;
      model: string;
    }) => saveAIConfig(baseURL, model),
    onMutate: async ({ baseURL, model }) => {
      await queryClient.cancelQueries({ queryKey: baseURLKey });
      await queryClient.cancelQueries({ queryKey: modelKey });
      const previousBaseURL = queryClient.getQueryData<string | null | undefined>(
        baseURLKey,
      );
      const previousModel = queryClient.getQueryData<string | null | undefined>(
        modelKey,
      );
      queryClient.setQueryData(baseURLKey, JSON.stringify(baseURL));
      queryClient.setQueryData(modelKey, JSON.stringify(model));
      return { previousBaseURL, previousModel };
    },
    onError: (_error, _variables, context) => {
      if (!context) {
        return;
      }
      queryClient.setQueryData(baseURLKey, context.previousBaseURL);
      queryClient.setQueryData(modelKey, context.previousModel);
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: baseURLKey });
      void queryClient.invalidateQueries({ queryKey: modelKey });
    },
  });
}

export function useIntegrationConnections() {
  return useQuery({
    queryKey: shietQueryKeys.connections(),
    queryFn: listIntegrationConnections,
  });
}

export function useConnectGoogle() {
  const queryClient = useQueryClient();
  const refreshGoogleQueries = () => {
    void queryClient.invalidateQueries({
      queryKey: shietQueryKeys.connections(),
    });
    void queryClient.invalidateQueries({
      queryKey: shietQueryKeys.calendars(),
    });
    void queryClient.invalidateQueries({
      queryKey: shietQueryKeys.selectedCalendars(),
    });
  };

  return useMutation({
    mutationFn: ({
      accountID,
      accountLabel,
    }: {
      accountID: string;
      accountLabel: string;
    }) => connectGoogle(accountID, accountLabel),
    onSettled: refreshGoogleQueries,
  });
}

export function useDisconnectGoogle() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (accountID: string) => disconnectGoogle(accountID),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.connections(),
      });
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.calendars(),
      });
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.selectedCalendars(),
      });
    },
  });
}

export function useSetCalendarSelected() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      calendarID,
      selected,
    }: {
      calendarID: number;
      selected: boolean;
    }) => setCalendarSelected(calendarID, selected),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.calendars(),
      });
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.selectedCalendars(),
      });
    },
  });
}

export function useSetCalendarDefaultCategory() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      calendarID,
      categoryID,
    }: {
      calendarID: number;
      categoryID: number | null;
    }) => setCalendarDefaultCategory(calendarID, categoryID),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.calendars(),
      });
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.selectedCalendars(),
      });
    },
  });
}

export function useSyncPeriod() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (periodID: number) => syncPeriod(periodID),
    onSuccess: (_result, periodID) => {
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.periods(),
      });
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.period(periodID),
      });
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.periodEvents(periodID),
      });
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.periodEventCategoryOverlays(periodID),
      });
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.periodReviewItems(periodID),
      });
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.gapTimeline(periodID),
      });
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.calendars(),
      });
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.connections(),
      });
    },
  });
}
