import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  classifyAIEndpoint,
  computeGaps,
  connectGoogle,
  createGapFill,
  createManualEvent,
  deleteManualEvent,
  disconnectGoogle,
  discoverLocalAIEndpoints,
  ensureCurrentPeriod,
  getSetting,
  listAIModels,
  listCalendars,
  listCategories,
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
  updateManualEvent,
  validateAIConfig,
} from "./clockrService";
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

export const clockrQueryKeys = {
  all: ["clockr"] as const,
  calendars: () => [...clockrQueryKeys.all, "calendars"] as const,
  categories: () => [...clockrQueryKeys.all, "categories"] as const,
  gapTimeline: (periodId: number) =>
    [...clockrQueryKeys.period(periodId), "gapTimeline"] as const,
  period: (periodId: number) =>
    [...clockrQueryKeys.periods(), periodId] as const,
  currentPeriod: (today: string, ianaTz: string) =>
    [...clockrQueryKeys.periods(), "current", today, ianaTz] as const,
  periods: () => [...clockrQueryKeys.all, "periods"] as const,
  periodEvents: (periodId: number) =>
    [...clockrQueryKeys.period(periodId), "events"] as const,
  periodGapFills: (periodId: number) =>
    [...clockrQueryKeys.period(periodId), "gapFills"] as const,
  periodReviewItems: (periodId: number) =>
    [...clockrQueryKeys.period(periodId), "reviewItems"] as const,
  periodTzSegments: (periodId: number) =>
    [...clockrQueryKeys.period(periodId), "tzSegments"] as const,
  selectedCalendars: () =>
    [...clockrQueryKeys.calendars(), "selected"] as const,
  connections: () => [...clockrQueryKeys.all, "connections"] as const,
  setting: (key: string) => [...clockrQueryKeys.all, "settings", key] as const,
  aiDiscovery: () => [...clockrQueryKeys.all, "ai", "discovery"] as const,
  aiClassification: (baseURL: string) =>
    [...clockrQueryKeys.all, "ai", "classification", baseURL] as const,
  aiModels: (baseURL: string) =>
    [...clockrQueryKeys.all, "ai", "models", baseURL] as const,
  aiValidation: (baseURL: string, apiKey: string, model: string) =>
    [...clockrQueryKeys.all, "ai", "validation", baseURL, apiKey, model] as const,
};

export function usePeriods() {
  return useQuery({
    queryKey: clockrQueryKeys.periods(),
    queryFn: listPeriods,
  });
}

export function useCurrentPeriod(today: string, ianaTz: string) {
  return useQuery({
    queryKey: clockrQueryKeys.currentPeriod(today, ianaTz),
    queryFn: () => ensureCurrentPeriod(today, ianaTz),
  });
}

export function useCategories() {
  return useQuery({
    queryKey: clockrQueryKeys.categories(),
    queryFn: listCategories,
  });
}

export function useCalendars() {
  return useQuery({
    queryKey: clockrQueryKeys.calendars(),
    queryFn: listCalendars,
  });
}

export function useSelectedCalendars() {
  return useQuery({
    queryKey: clockrQueryKeys.selectedCalendars(),
    queryFn: listSelectedCalendars,
  });
}

export function useEvents(periodId: number | null | undefined) {
  return useQuery({
    enabled: typeof periodId === "number",
    queryKey: clockrQueryKeys.periodEvents(periodId ?? 0),
    queryFn: () => listEvents(periodId as number),
  });
}

export function useGapFills(periodId: number | null | undefined) {
  return useQuery({
    enabled: typeof periodId === "number",
    queryKey: clockrQueryKeys.periodGapFills(periodId ?? 0),
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
        queryKey: clockrQueryKeys.periodGapFills(periodId),
      });
      void queryClient.invalidateQueries({
        queryKey: clockrQueryKeys.gapTimeline(periodId),
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
        queryKey: clockrQueryKeys.periodGapFills(periodId),
      });
      void queryClient.invalidateQueries({
        queryKey: clockrQueryKeys.gapTimeline(periodId),
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
        queryKey: clockrQueryKeys.periodGapFills(periodId),
      });
      void queryClient.invalidateQueries({
        queryKey: clockrQueryKeys.gapTimeline(periodId),
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
        queryKey: clockrQueryKeys.periodGapFills(periodId),
      });
      void queryClient.invalidateQueries({
        queryKey: clockrQueryKeys.gapTimeline(periodId),
      });
    },
  });
}

export function useOpenReviewItems(periodId: number | null | undefined) {
  return useQuery({
    enabled: typeof periodId === "number",
    queryKey: clockrQueryKeys.periodReviewItems(periodId ?? 0),
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
        queryKey: clockrQueryKeys.periodReviewItems(periodId),
      });
      void queryClient.invalidateQueries({
        queryKey: clockrQueryKeys.periodEvents(periodId),
      });
      void queryClient.invalidateQueries({
        queryKey: clockrQueryKeys.periodGapFills(periodId),
      });
      void queryClient.invalidateQueries({
        queryKey: clockrQueryKeys.gapTimeline(periodId),
      });
    },
  });
}

export function useTzSegments(periodId: number | null | undefined) {
  return useQuery({
    enabled: typeof periodId === "number",
    queryKey: clockrQueryKeys.periodTzSegments(periodId ?? 0),
    queryFn: () => listTzSegments(periodId as number),
  });
}

export function useGapTimeline(periodId: number | null | undefined) {
  return useQuery({
    enabled: typeof periodId === "number",
    queryKey: clockrQueryKeys.gapTimeline(periodId ?? 0),
    queryFn: () => computeGaps(periodId as number),
  });
}

export function useSetting(key: string) {
  return useQuery({
    queryKey: clockrQueryKeys.setting(key),
    queryFn: () => getSetting(key),
  });
}

export function useSetSetting() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ key, value }: { key: string; value: string }) =>
      setSetting(key, value),
    onMutate: async ({ key, value }) => {
      const queryKey = clockrQueryKeys.setting(key);
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
        clockrQueryKeys.setting(context.key),
        context.previous,
      );
    },
    onSettled: (_result, _error, { key }) => {
      void queryClient.invalidateQueries({
        queryKey: clockrQueryKeys.setting(key),
      });
    },
  });
}

export function useDiscoverLocalAIEndpoints() {
  return useQuery({
    queryKey: clockrQueryKeys.aiDiscovery(),
    queryFn: discoverLocalAIEndpoints,
  });
}

export function useClassifyAIEndpoint(baseURL: string) {
  return useQuery({
    enabled: baseURL.trim().length > 0,
    queryKey: clockrQueryKeys.aiClassification(baseURL),
    queryFn: () => classifyAIEndpoint(baseURL),
  });
}

export function useAIModels(baseURL: string, apiKey: string) {
  return useQuery({
    enabled: baseURL.trim().length > 0,
    queryKey: clockrQueryKeys.aiModels(baseURL),
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
    queryKey: clockrQueryKeys.aiValidation(baseURL, apiKey, model),
    queryFn: () => validateAIConfig(baseURL, apiKey, model),
    retry: false,
  });
}

export function useClearAIModel() {
  const queryClient = useQueryClient();
  const queryKey = clockrQueryKeys.setting("ai.model");

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
  const queryKey = clockrQueryKeys.setting("ai.base_url");

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
  const queryKey = clockrQueryKeys.setting("ai.model");

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
  const baseURLKey = clockrQueryKeys.setting("ai.base_url");
  const modelKey = clockrQueryKeys.setting("ai.model");

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
    queryKey: clockrQueryKeys.connections(),
    queryFn: listIntegrationConnections,
  });
}

export function useConnectGoogle() {
  const queryClient = useQueryClient();
  const refreshGoogleQueries = () => {
    void queryClient.invalidateQueries({
      queryKey: clockrQueryKeys.connections(),
    });
    void queryClient.invalidateQueries({
      queryKey: clockrQueryKeys.calendars(),
    });
    void queryClient.invalidateQueries({
      queryKey: clockrQueryKeys.selectedCalendars(),
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
        queryKey: clockrQueryKeys.connections(),
      });
      void queryClient.invalidateQueries({
        queryKey: clockrQueryKeys.calendars(),
      });
      void queryClient.invalidateQueries({
        queryKey: clockrQueryKeys.selectedCalendars(),
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
        queryKey: clockrQueryKeys.calendars(),
      });
      void queryClient.invalidateQueries({
        queryKey: clockrQueryKeys.selectedCalendars(),
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
        queryKey: clockrQueryKeys.calendars(),
      });
      void queryClient.invalidateQueries({
        queryKey: clockrQueryKeys.selectedCalendars(),
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
        queryKey: clockrQueryKeys.periods(),
      });
      void queryClient.invalidateQueries({
        queryKey: clockrQueryKeys.period(periodID),
      });
      void queryClient.invalidateQueries({
        queryKey: clockrQueryKeys.periodEvents(periodID),
      });
      void queryClient.invalidateQueries({
        queryKey: clockrQueryKeys.periodReviewItems(periodID),
      });
      void queryClient.invalidateQueries({
        queryKey: clockrQueryKeys.gapTimeline(periodID),
      });
      void queryClient.invalidateQueries({
        queryKey: clockrQueryKeys.calendars(),
      });
      void queryClient.invalidateQueries({
        queryKey: clockrQueryKeys.connections(),
      });
    },
  });
}
