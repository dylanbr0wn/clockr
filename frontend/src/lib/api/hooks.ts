import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  classifyAIEndpoint,
  computeGaps,
  connectGitHub,
  connectIntegration,
  connectSlack,
  connectBitbucket,
  createGapTimeEntry,
  createCategory,
  createTimeEntry,
  deleteCategory,
  archiveCategory,
  deleteTimeEntry,
  disconnectGitHub,
  disconnectIntegration,
  disconnectSlack,
  disconnectBitbucket,
  discoverLocalAIEndpoints,
  ensureCurrentPeriod,
  excludeEvent,
  getIntegrationAuthStatus,
  getLogPath,
  getSetting,
  listAIModels,
  listCalendars,
  listCategories,
  listEventCategoryOverlays,
  listEvents,
  listTimeEntries,
  listGitHubRepos,
  listSlackChannels,
  listBitbucketWorkspaces,
  listBitbucketRepos,
  listExportTemplates,
  createExportTemplate,
  updateExportTemplate,
  deleteExportTemplate,
  duplicateExportTemplate,
  previewExport,
  listIntegrationConnections,
  listIntegrationProviders,
  listReviewDecisions,
  listPeriods,
  listSelectedCalendars,
  listTzSegments,
  refreshGitHubRepos,
  refreshSlackChannels,
  refreshBitbucketResources,
  resolveReviewDecision,
  listGapEvidence,
  revealLogFolder,
  saveAIConfig,
  saveAIEndpoint,
  saveAIModel,
  saveAIAPIKey,
  clearAIAPIKey,
  hasAIAPIKey,
  setCalendarDefaultCategory,
  setCalendarSelected,
  setGitHubRepoSelected,
  setSlackChannelSelected,
  setBitbucketWorkspaceSelected,
  setBitbucketRepoSelected,
  setSetting,
  suggestGapFill,
  syncPeriod,
  updateCategory,
  updateTimeEntry,
  validateAIConfig,
  expectedTimeForRange,
  listWorkSchedules,
  replaceActiveWorkSchedule,
  listScheduleExceptions,
  upsertScheduleException,
  deleteScheduleException,
} from "./shietService";
import type { TimeWindow, WorkSchedule } from "./types";

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
  categories: (includeArchived = false) =>
    [...shietQueryKeys.all, "categories", includeArchived ? "all" : "active"] as const,
  exportTemplates: () => [...shietQueryKeys.all, "exportTemplates"] as const,
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
  periodTimeEntries: (periodId: number) =>
    [...shietQueryKeys.period(periodId), "timeEntries"] as const,
  periodReviewDecisions: (periodId: number) =>
    [...shietQueryKeys.period(periodId), "reviewDecisions"] as const,
  periodTzSegments: (periodId: number) =>
    [...shietQueryKeys.period(periodId), "tzSegments"] as const,
  expectedTimeRange: (startDate: string, endDate: string) =>
    [...shietQueryKeys.all, "expectedTime", startDate, endDate] as const,
  workSchedules: () => [...shietQueryKeys.all, "workSchedules"] as const,
  scheduleExceptions: () =>
    [...shietQueryKeys.all, "scheduleExceptions"] as const,
  selectedCalendars: () =>
    [...shietQueryKeys.calendars(), "selected"] as const,
  connections: () => [...shietQueryKeys.all, "connections"] as const,
  integrationProviders: () =>
    [...shietQueryKeys.all, "integrationProviders"] as const,
  integrationAuthStatus: (provider: string) =>
    [...shietQueryKeys.all, "integrationAuthStatus", provider] as const,
  googleAuthStatus: () =>
    [...shietQueryKeys.integrationAuthStatus("google")] as const,
  logPath: () => [...shietQueryKeys.all, "logPath"] as const,
  githubRepos: () => [...shietQueryKeys.all, "githubRepos"] as const,
  githubAuthMode: () =>
    [...shietQueryKeys.integrationAuthStatus("github")] as const,
  githubOAuthAvailable: () =>
    [...shietQueryKeys.integrationAuthStatus("github")] as const,
  slackChannels: () => [...shietQueryKeys.all, "slackChannels"] as const,
  slackAuthMode: () =>
    [...shietQueryKeys.integrationAuthStatus("slack")] as const,
  slackOAuthAvailable: () =>
    [...shietQueryKeys.integrationAuthStatus("slack")] as const,
  bitbucketWorkspaces: () => [...shietQueryKeys.all, "bitbucketWorkspaces"] as const,
  bitbucketRepos: () => [...shietQueryKeys.all, "bitbucketRepos"] as const,
  bitbucketAuthMode: () =>
    [...shietQueryKeys.integrationAuthStatus("bitbucket")] as const,
  bitbucketOAuthAvailable: () =>
    [...shietQueryKeys.integrationAuthStatus("bitbucket")] as const,
  setting: (key: string) => [...shietQueryKeys.all, "settings", key] as const,
  aiDiscovery: () => [...shietQueryKeys.all, "ai", "discovery"] as const,
  aiClassification: (baseURL: string) =>
    [...shietQueryKeys.all, "ai", "classification", baseURL] as const,
  aiModels: (baseURL: string) =>
    [...shietQueryKeys.all, "ai", "models", baseURL] as const,
  aiValidation: (baseURL: string, apiKey: string, model: string) =>
    [...shietQueryKeys.all, "ai", "validation", baseURL, apiKey, model] as const,
  aiHasKey: () => [...shietQueryKeys.all, "ai", "hasKey"] as const,
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

export function useCategories(includeArchived = false) {
  return useQuery({
    queryKey: shietQueryKeys.categories(includeArchived),
    queryFn: () => listCategories(includeArchived),
  });
}

export function useExportTemplates() {
  return useQuery({
    queryKey: shietQueryKeys.exportTemplates(),
    queryFn: listExportTemplates,
  });
}

function invalidateExportTemplateQueries(
  queryClient: ReturnType<typeof useQueryClient>,
) {
  void queryClient.invalidateQueries({
    queryKey: shietQueryKeys.exportTemplates(),
  });
}

export function useCreateExportTemplate() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: createExportTemplate,
    onSuccess: () => {
      invalidateExportTemplateQueries(queryClient);
    },
  });
}

export function useUpdateExportTemplate() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: updateExportTemplate,
    onSuccess: () => {
      invalidateExportTemplateQueries(queryClient);
    },
  });
}

export function useDeleteExportTemplate() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: deleteExportTemplate,
    onSuccess: () => {
      invalidateExportTemplateQueries(queryClient);
    },
  });
}

export function useDuplicateExportTemplate() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: duplicateExportTemplate,
    onSuccess: () => {
      invalidateExportTemplateQueries(queryClient);
    },
  });
}

export function usePreviewExport(
  input: {
    periodId: number | null | undefined;
    templateKey?: string;
    format?: string;
    body?: string;
  },
  enabled = true,
) {
  return useQuery({
    enabled:
      enabled &&
      typeof input.periodId === "number" &&
      (Boolean(input.body?.trim()) || Boolean(input.templateKey)),
    queryKey: [
      ...shietQueryKeys.exportTemplates(),
      "preview",
      input.periodId,
      input.templateKey ?? "",
      input.format ?? "",
      input.body ?? "",
    ],
    queryFn: () =>
      previewExport({
        periodId: input.periodId as number,
        templateKey: input.templateKey,
        format: input.format,
        body: input.body,
      }),
  });
}

function invalidateCategoryQueries(queryClient: ReturnType<typeof useQueryClient>) {
  void queryClient.invalidateQueries({
    queryKey: [...shietQueryKeys.all, "categories"],
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

export function useArchiveCategory() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: archiveCategory,
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

export function useExpectedTimeForRange(
  startDate: string | null | undefined,
  endDate: string | null | undefined,
) {
  return useQuery({
    enabled: Boolean(startDate && endDate),
    queryKey: shietQueryKeys.expectedTimeRange(startDate ?? "", endDate ?? ""),
    queryFn: () => expectedTimeForRange(startDate as string, endDate as string),
  });
}

function pickActiveWorkSchedule(
  schedules: WorkSchedule[] | undefined,
): WorkSchedule | null {
  if (!schedules?.length) {
    return null;
  }

  const openEnded = schedules
    .filter((schedule) => !schedule.effectiveTo)
    .sort((a, b) => b.effectiveFrom.localeCompare(a.effectiveFrom));
  if (openEnded[0]) {
    return openEnded[0];
  }

  return [...schedules].sort((a, b) =>
    b.effectiveFrom.localeCompare(a.effectiveFrom),
  )[0];
}

export function useWorkSchedules() {
  return useQuery({
    queryKey: shietQueryKeys.workSchedules(),
    queryFn: listWorkSchedules,
  });
}

export function useActiveWorkSchedule() {
  const query = useWorkSchedules();
  return {
    ...query,
    data: pickActiveWorkSchedule(query.data),
  };
}

export function useReplaceActiveWorkSchedule() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: replaceActiveWorkSchedule,
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.workSchedules(),
      });
      void queryClient.invalidateQueries({
        queryKey: [...shietQueryKeys.all, "expectedTime"],
      });
    },
  });
}

export function useScheduleExceptions() {
  return useQuery({
    queryKey: shietQueryKeys.scheduleExceptions(),
    queryFn: listScheduleExceptions,
  });
}

export function useUpsertScheduleException() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: upsertScheduleException,
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.scheduleExceptions(),
      });
      void queryClient.invalidateQueries({
        queryKey: [...shietQueryKeys.all, "expectedTime"],
      });
    },
  });
}

export function useDeleteScheduleException() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: deleteScheduleException,
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.scheduleExceptions(),
      });
      void queryClient.invalidateQueries({
        queryKey: [...shietQueryKeys.all, "expectedTime"],
      });
    },
  });
}

export function useTimeEntries(periodId: number | null | undefined) {
  return useQuery({
    enabled: typeof periodId === "number",
    queryKey: shietQueryKeys.periodTimeEntries(periodId ?? 0),
    queryFn: () => listTimeEntries(periodId as number),
  });
}

export function useCreateTimeEntry() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: createTimeEntry,
    onSuccess: (timeEntry, input) => {
      const periodId = timeEntry.periodId || input.periodId;

      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.periodTimeEntries(periodId),
      });
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.gapTimeline(periodId),
      });
    },
  });
}

export function useCreateGapTimeEntry() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: createGapTimeEntry,
    onSuccess: (timeEntry, input) => {
      const periodId = timeEntry.periodId || input.periodId;

      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.periodTimeEntries(periodId),
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

export function useListGapEvidence() {
  return useMutation({
    mutationFn: (window: TimeWindow) => listGapEvidence(window),
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

export function useUpdateTimeEntry() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: updateTimeEntry,
    onSuccess: (timeEntry, input) => {
      const periodId = timeEntry.periodId || input.periodId;

      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.periodTimeEntries(periodId),
      });
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.gapTimeline(periodId),
      });
    },
  });
}

export function useDeleteTimeEntry() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: deleteTimeEntry,
    onSuccess: (result, input) => {
      const periodId = result.periodId || input.periodId;

      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.periodTimeEntries(periodId),
      });
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.gapTimeline(periodId),
      });
    },
  });
}

export function useReviewDecisions(periodId: number | null | undefined) {
  return useQuery({
    enabled: typeof periodId === "number",
    queryKey: shietQueryKeys.periodReviewDecisions(periodId ?? 0),
    queryFn: () => listReviewDecisions(periodId as number),
  });
}

export function useResolveReviewDecision() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: resolveReviewDecision,
    onSuccess: (result) => {
      const periodId = result.periodId;
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.periodReviewDecisions(periodId),
      });
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.periodEvents(periodId),
      });
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.periodTimeEntries(periodId),
      });
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.gapTimeline(periodId),
      });
    },
  });
}

export function useExcludeEvent() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: excludeEvent,
    onSuccess: (result) => {
      const periodId = result.periodId;
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.periodEvents(periodId),
      });
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.periodReviewDecisions(periodId),
      });
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.periodEventCategoryOverlays(periodId),
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

export function useHasAIAPIKey() {
  return useQuery({
    queryKey: shietQueryKeys.aiHasKey(),
    queryFn: hasAIAPIKey,
  });
}

export function useSaveAIAPIKey() {
  const queryClient = useQueryClient();
  const queryKey = shietQueryKeys.aiHasKey();

  return useMutation({
    mutationFn: (apiKey: string) => saveAIAPIKey(apiKey),
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey });
    },
  });
}

export function useClearAIAPIKey() {
  const queryClient = useQueryClient();
  const queryKey = shietQueryKeys.aiHasKey();

  return useMutation({
    mutationFn: () => clearAIAPIKey(),
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey });
    },
  });
}

export function useIntegrationConnections() {
  return useQuery({
    queryKey: shietQueryKeys.connections(),
    queryFn: listIntegrationConnections,
  });
}

export function useIntegrationProviders() {
  return useQuery({
    queryKey: shietQueryKeys.integrationProviders(),
    queryFn: listIntegrationProviders,
  });
}

export function useIntegrationAuthStatus(
  provider: string,
  options?: { enabled?: boolean },
) {
  return useQuery({
    queryKey: shietQueryKeys.integrationAuthStatus(provider),
    queryFn: () => getIntegrationAuthStatus(provider),
    enabled: options?.enabled ?? true,
  });
}

function invalidateProviderIntegrationQueries(
  queryClient: ReturnType<typeof useQueryClient>,
  provider: string,
) {
  void queryClient.invalidateQueries({
    queryKey: shietQueryKeys.integrationAuthStatus(provider),
  });
  void queryClient.invalidateQueries({
    queryKey: shietQueryKeys.connections(),
  });
  if (provider === "google") {
    void queryClient.invalidateQueries({
      queryKey: shietQueryKeys.calendars(),
    });
    void queryClient.invalidateQueries({
      queryKey: shietQueryKeys.selectedCalendars(),
    });
  }
  if (provider === "github") {
    void queryClient.invalidateQueries({
      queryKey: shietQueryKeys.githubRepos(),
    });
  }
  if (provider === "slack") {
    void queryClient.invalidateQueries({
      queryKey: shietQueryKeys.slackChannels(),
    });
  }
  if (provider === "bitbucket") {
    void queryClient.invalidateQueries({
      queryKey: shietQueryKeys.bitbucketWorkspaces(),
    });
    void queryClient.invalidateQueries({
      queryKey: shietQueryKeys.bitbucketRepos(),
    });
  }
}

export function useConnectIntegration() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: connectIntegration,
    onSettled: (_data, _error, input) => {
      invalidateProviderIntegrationQueries(queryClient, input.provider);
    },
  });
}

export function useDisconnectIntegration() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      provider,
      accountID,
    }: {
      provider: string;
      accountID: string;
    }) => disconnectIntegration(provider, accountID),
    onSuccess: (_data, input) => {
      invalidateProviderIntegrationQueries(queryClient, input.provider);
    },
  });
}

export function useLogPath() {
  return useQuery({
    queryKey: shietQueryKeys.logPath(),
    queryFn: getLogPath,
  });
}

export function useRevealLogFolder() {
  return useMutation({
    mutationFn: revealLogFolder,
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

export function useGitHubRepos() {
  return useQuery({
    queryKey: shietQueryKeys.githubRepos(),
    queryFn: listGitHubRepos,
  });
}

export function useGitHubAuthMode() {
  return useQuery({
    queryKey: shietQueryKeys.integrationAuthStatus("github"),
    queryFn: () => getIntegrationAuthStatus("github"),
    select: (status) => status.mode,
  });
}

export function useGitHubOAuthAvailable() {
  return useQuery({
    queryKey: shietQueryKeys.integrationAuthStatus("github"),
    queryFn: () => getIntegrationAuthStatus("github"),
    select: (status) => status.oauthAvailable,
  });
}

export function useConnectGitHub() {
  const queryClient = useQueryClient();
  const refreshGitHubQueries = () => {
    invalidateProviderIntegrationQueries(queryClient, "github");
  };

  return useMutation({
    mutationFn: (pat: string) => connectGitHub(pat),
    onSettled: refreshGitHubQueries,
  });
}

export function useDisconnectGitHub() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (accountID: string) => disconnectGitHub(accountID),
    onSuccess: () => {
      invalidateProviderIntegrationQueries(queryClient, "github");
    },
  });
}

export function useSetGitHubRepoSelected() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      repoID,
      selected,
    }: {
      repoID: number;
      selected: boolean;
    }) => setGitHubRepoSelected(repoID, selected),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.githubRepos(),
      });
    },
  });
}

export function useRefreshGitHubRepos() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (accountID: string) => refreshGitHubRepos(accountID),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.githubRepos(),
      });
    },
  });
}

export function useSlackChannels() {
  return useQuery({
    queryKey: shietQueryKeys.slackChannels(),
    queryFn: listSlackChannels,
  });
}

export function useSlackAuthMode() {
  return useQuery({
    queryKey: shietQueryKeys.integrationAuthStatus("slack"),
    queryFn: () => getIntegrationAuthStatus("slack"),
    select: (status) => status.mode,
  });
}

export function useSlackOAuthAvailable() {
  return useQuery({
    queryKey: shietQueryKeys.integrationAuthStatus("slack"),
    queryFn: () => getIntegrationAuthStatus("slack"),
    select: (status) => status.oauthAvailable,
  });
}

export function useConnectSlack() {
  const queryClient = useQueryClient();
  const refreshSlackQueries = () => {
    invalidateProviderIntegrationQueries(queryClient, "slack");
  };

  return useMutation({
    mutationFn: () => connectSlack(),
    onSettled: refreshSlackQueries,
  });
}

export function useDisconnectSlack() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (accountID: string) => disconnectSlack(accountID),
    onSuccess: () => {
      invalidateProviderIntegrationQueries(queryClient, "slack");
    },
  });
}

export function useSetSlackChannelSelected() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      channelID,
      selected,
    }: {
      channelID: number;
      selected: boolean;
    }) => setSlackChannelSelected(channelID, selected),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.slackChannels(),
      });
    },
  });
}

export function useRefreshSlackChannels() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (accountID: string) => refreshSlackChannels(accountID),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.slackChannels(),
      });
    },
  });
}

export function useBitbucketWorkspaces() {
  return useQuery({
    queryKey: shietQueryKeys.bitbucketWorkspaces(),
    queryFn: listBitbucketWorkspaces,
  });
}

export function useBitbucketRepos() {
  return useQuery({
    queryKey: shietQueryKeys.bitbucketRepos(),
    queryFn: listBitbucketRepos,
  });
}

export function useConnectBitbucket() {
  const queryClient = useQueryClient();
  const refreshBitbucketQueries = () => {
    invalidateProviderIntegrationQueries(queryClient, "bitbucket");
  };

  return useMutation({
    mutationFn: () => connectBitbucket(),
    onSettled: refreshBitbucketQueries,
  });
}

export function useDisconnectBitbucket() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (accountID: string) => disconnectBitbucket(accountID),
    onSuccess: () => {
      invalidateProviderIntegrationQueries(queryClient, "bitbucket");
    },
  });
}

export function useSetBitbucketWorkspaceSelected() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      workspaceID,
      selected,
    }: {
      workspaceID: number;
      selected: boolean;
    }) => setBitbucketWorkspaceSelected(workspaceID, selected),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.bitbucketWorkspaces(),
      });
    },
  });
}

export function useSetBitbucketRepoSelected() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      repoID,
      selected,
    }: {
      repoID: number;
      selected: boolean;
    }) => setBitbucketRepoSelected(repoID, selected),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.bitbucketRepos(),
      });
    },
  });
}

export function useRefreshBitbucketResources() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (accountID: string) => refreshBitbucketResources(accountID),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.bitbucketWorkspaces(),
      });
      void queryClient.invalidateQueries({
        queryKey: shietQueryKeys.bitbucketRepos(),
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
        queryKey: shietQueryKeys.periodReviewDecisions(periodID),
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
