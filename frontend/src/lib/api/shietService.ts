import * as generatedApp from "../../../wailsjs/go/main/App";
import type {
  AIClassification,
  AIEndpoint,
  AIValidationResult,
  Calendar,
  Category,
  CreateCategoryInput,
  CreateProjectInput,
  DayTimeline,
  Event,
  EventCategoryOverlay,
  ExcludeEventInput,
  ExpectedTime,
  GapEvidenceItem,
  GitHubRepo,
  BitbucketRepo,
  BitbucketWorkspace,
  GoogleAuthStatus,
  IntegrationAuthStatus,
  IntegrationConnection,
  IntegrationProvider,
  ConnectIntegrationInput,
  Project,
  TimeEntry,
  TimeEntryDeleteInput,
  TimeEntryInput,
  TimeEntryUpdateInput,
  Period,
  PeriodExportModel,
  ReviewDecision,
  ResolveReviewDecisionInput,
  ScheduleException,
  SlackChannel,
  TimeWindow,
  TzSegment,
  UpdateCategoryInput,
  UpdateProjectInput,
  ExportTemplate,
  CreateExportTemplateInput,
  UpdateExportTemplateInput,
  PreviewExportInput,
  ExportFieldInfo,
  WorkSchedule,
  WorkScheduleDay,
  WorkingWindow,
} from "./types";
import {
  ensureCurrentPeriodRPC,
  getPeriodByRangeRPC,
  getPeriodRPC,
  listPeriodsRPC,
} from "./periodRpc";
import {
  buildPeriodExportRPC,
  computeGapsRPC,
  connectIntegrationRPC,
  createCategoryRPC,
  createExportTemplateRPC,
  createGapTimeEntryRPC,
  createProjectRPC,
  createTimeEntryRPC,
  deleteCategoryRPC,
  archiveCategoryRPC,
  archiveProjectRPC,
  deleteExportTemplateRPC,
  deleteProjectRPC,
  deleteTimeEntryRPC,
  disconnectIntegrationRPC,
  duplicateExportTemplateRPC,
  excludeEventRPC,
  getCategoryRPC,
  getEventRPC,
  getExportTemplateRPC,
  getIntegrationAuthStatusRPC,
  getProjectRPC,
  getSettingRPC,
  listCalendarsRPC,
  listCategoriesRPC,
  listEventCategoryOverlaysRPC,
  listEventsRPC,
  listExportFieldCatalogRPC,
  listExportTemplatesRPC,
  listTimeEntriesRPC,
  listGapEvidenceRPC,
  listGitHubReposRPC,
  listBitbucketWorkspacesRPC,
  listBitbucketReposRPC,
  listIntegrationConnectionsRPC,
  listIntegrationProvidersRPC,
  listProjectsRPC,
  listReviewDecisionsRPC,
  listSelectedCalendarsRPC,
  listSlackChannelsRPC,
  listTzSegmentsRPC,
  previewExportRPC,
  refreshGitHubReposRPC,
  refreshSlackChannelsRPC,
  refreshBitbucketResourcesRPC,
  renderPeriodExportRPC,
  resolveReviewDecisionRPC,
  setCalendarDefaultCategoryRPC,
  setCalendarSelectedRPC,
  setGitHubRepoSelectedRPC,
  setSettingRPC,
  setSlackChannelSelectedRPC,
  setBitbucketWorkspaceSelectedRPC,
  setBitbucketRepoSelectedRPC,
  suggestGapFillRPC,
  syncPeriodRPC,
  updateCategoryRPC,
  updateExportTemplateRPC,
  updateProjectRPC,
  updateTimeEntryRPC,
} from "./applicationRpc";
import {
  deleteScheduleExceptionRPC,
  expectedTimeForRangeRPC,
  listScheduleExceptionsRPC,
  listWorkSchedulesRPC,
  replaceActiveWorkScheduleRPC,
  upsertScheduleExceptionRPC,
} from "./workScheduleRpc";

interface ShietApp {
  ClassifyAIEndpoint(baseURL: string): Promise<AIClassification>;
  ConnectGitHub(pat: string): Promise<IntegrationConnection>;
  ConnectGoogle(accountID: string, accountLabel: string): Promise<IntegrationConnection>;
  ConnectSlack(): Promise<IntegrationConnection>;
  DisconnectGitHub(accountID: string): Promise<void>;
  DisconnectGoogle(accountID: string): Promise<void>;
  DisconnectSlack(accountID: string): Promise<void>;
  DiscoverLocalAIEndpoints(): Promise<AIEndpoint[]>;
  GetGoogleAuthStatus(): Promise<GoogleAuthStatus>;
  GitHubAuthMode(): Promise<string>;
  GitHubOAuthAvailable(): Promise<boolean>;
  ListAIModels(baseURL: string, apiKey: string): Promise<string[]>;
  LogPath(): Promise<string>;
  RevealLogFolder(): Promise<void>;
  SlackAuthMode(): Promise<string>;
  SlackOAuthAvailable(): Promise<boolean>;
  SaveAIConfig(baseURL: string, model: string): Promise<void>;
  SaveAIEndpoint(baseURL: string): Promise<void>;
  SaveAIModel(model: string): Promise<void>;
  SaveAIAPIKey(apiKey: string): Promise<void>;
  ClearAIAPIKey(): Promise<void>;
  HasAIAPIKey(): Promise<boolean>;
  SaveExportFile(defaultFilename: string, content: string): Promise<string>;
  ValidateAIConfig(
    baseURL: string,
    apiKey: string,
    model: string,
  ): Promise<AIValidationResult>;
}

declare global {
  interface Window {
    go?: {
      main?: {
        App?: unknown;
      };
    };
  }
}

const appBackend = generatedApp as unknown as ShietApp;

export function isShietAppAvailable() {
  return Boolean(
    typeof window !== "undefined" &&
      window.go?.main?.App,
  );
}

async function readFromBackend<T>(fallback: T, read: () => Promise<T>) {
  if (!isShietAppAvailable()) {
    return fallback;
  }

  return read();
}

async function readFromPortableBackend<T>(fallback: T, read: () => Promise<T>) {
	if (!isPortableBackendAvailable()) {
		return fallback;
	}
	return read();
}

function isPortableBackendAvailable() {
  // Prefer Wails bindings when present, but do not require them: Connect on
  // same-origin /rpc works even when a path-based deep link skipped IPC
  // injection (window.go missing). Hash routing is the primary fix; this is
  // a safety net for production AssetServer builds.
  if (isShietAppAvailable()) {
    return true;
  }
  if (import.meta.env.VITE_SHIET_RPC_BASE_URL?.trim()) {
    return true;
  }
  return import.meta.env.PROD;
}

async function writeToPortableBackend<T>(write: () => Promise<T>) {
  if (!isPortableBackendAvailable()) throw new Error("shiet backend is unavailable");
  return write();
}

async function writeToBackend<T>(write: () => Promise<T>) {
  if (!isShietAppAvailable()) {
    throw new Error("shiet backend is unavailable");
  }

  return write();
}

export function listPeriods() {
  return readFromPortableBackend<Period[]>([], listPeriodsRPC);
}

export function getPeriod(id: number) {
  return readFromPortableBackend<Period | null>(null, () => getPeriodRPC(id));
}

export function getPeriodByRange(startDate: string, endDate: string) {
  return readFromPortableBackend<Period | null>(null, () =>
    getPeriodByRangeRPC(startDate, endDate),
  );
}

export function ensureCurrentPeriod(today: string, ianaTz: string) {
  return readFromPortableBackend<Period | null>(null, () =>
    ensureCurrentPeriodRPC(today, ianaTz),
  );
}

export function listCategories(includeArchived = false) {
  return readFromPortableBackend<Category[]>([], () =>
    listCategoriesRPC(includeArchived),
  );
}

export function getCategory(id: number) {
  return readFromPortableBackend<Category | null>(null, () => getCategoryRPC(id));
}

export function createCategory(input: CreateCategoryInput) {
  return writeToPortableBackend(() => createCategoryRPC(input));
}

export function updateCategory(input: UpdateCategoryInput) {
  return writeToPortableBackend(() => updateCategoryRPC(input));
}

export function deleteCategory(id: number) {
  return writeToPortableBackend(() => deleteCategoryRPC(id));
}

export function archiveCategory(id: number) {
  return writeToPortableBackend(() => archiveCategoryRPC(id));
}

export function listEventCategoryOverlays(periodId: number) {
  return readFromPortableBackend<EventCategoryOverlay[]>([], () => listEventCategoryOverlaysRPC(periodId));
}

export function listProjects(includeArchived = false) {
  return readFromPortableBackend<Project[]>([], () => listProjectsRPC(includeArchived));
}

export function getProject(id: number) {
  return readFromPortableBackend<Project | null>(null, () => getProjectRPC(id));
}

export function createProject(input: CreateProjectInput) {
  return writeToPortableBackend(() => createProjectRPC(input));
}

export function updateProject(input: UpdateProjectInput) {
  return writeToPortableBackend(() => updateProjectRPC(input));
}

export function deleteProject(id: number) {
  return writeToPortableBackend(() => deleteProjectRPC(id));
}

export function archiveProject(id: number) {
  return writeToPortableBackend(() => archiveProjectRPC(id));
}

export function listCalendars() {
  return readFromPortableBackend<Calendar[]>([], listCalendarsRPC);
}

export function listSelectedCalendars() {
  return readFromPortableBackend<Calendar[]>([], listSelectedCalendarsRPC);
}

export function listEvents(periodId: number) {
  return readFromPortableBackend<Event[]>([], () => listEventsRPC(periodId));
}

export function getEvent(id: number) {
  return readFromPortableBackend<Event | null>(null, () => getEventRPC(id));
}

export function listTimeEntries(periodId: number) {
  return readFromPortableBackend<TimeEntry[]>([], () => listTimeEntriesRPC(periodId));
}

export function createGapTimeEntry(input: TimeEntryInput) {
  return writeToPortableBackend(() => createGapTimeEntryRPC(input));
}

export function createTimeEntry(input: TimeEntryInput) {
  return writeToPortableBackend(() => createTimeEntryRPC(input));
}

export function updateTimeEntry(input: TimeEntryUpdateInput) {
  return writeToPortableBackend(() => updateTimeEntryRPC(input));
}

export function deleteTimeEntry(input: TimeEntryDeleteInput) {
  return writeToPortableBackend(() => deleteTimeEntryRPC(input));
}

export function listReviewDecisions(periodId: number) {
  return readFromPortableBackend<ReviewDecision[]>([], () => listReviewDecisionsRPC(periodId));
}

export function resolveReviewDecision(input: ResolveReviewDecisionInput) {
  return writeToPortableBackend(() => resolveReviewDecisionRPC(input));
}

export function excludeEvent(input: ExcludeEventInput) {
  return writeToPortableBackend(() => excludeEventRPC(input));
}

export function listTzSegments(periodId: number) {
  return readFromPortableBackend<TzSegment[]>([], () => listTzSegmentsRPC(periodId));
}

export function computeGaps(periodId: number) {
  return readFromPortableBackend<DayTimeline[]>([], () => computeGapsRPC(periodId));
}

export function expectedTimeForRange(startDate: string, endDate: string) {
  return readFromPortableBackend<ExpectedTime[]>([], () =>
    expectedTimeForRangeRPC(startDate, endDate),
  );
}

export function listWorkSchedules() {
  return readFromPortableBackend<WorkSchedule[]>([], () => listWorkSchedulesRPC());
}

export function replaceActiveWorkSchedule(input: {
  timezone: string;
  workweekStart: string;
  effectiveFrom: string;
  days: WorkScheduleDay[];
}) {
  return writeToPortableBackend(() => replaceActiveWorkScheduleRPC(input));
}

export function listScheduleExceptions() {
  return readFromPortableBackend<ScheduleException[]>([], () =>
    listScheduleExceptionsRPC(),
  );
}

export function upsertScheduleException(input: {
  date: string;
  kind: string;
  expectedMinutes: number;
  windows?: WorkingWindow[];
}) {
  return writeToPortableBackend(() => upsertScheduleExceptionRPC(input));
}

export function deleteScheduleException(date: string) {
  return writeToPortableBackend(() => deleteScheduleExceptionRPC(date));
}

export function getSetting(key: string) {
  return readFromPortableBackend<string | null>(
    localStorage.getItem(`shiet.setting.${key}`),
    () => getSettingRPC(key),
  );
}

export function setSetting(key: string, value: string) {
  if (!isPortableBackendAvailable()) {
    localStorage.setItem(`shiet.setting.${key}`, value);
    return Promise.resolve();
  }

  return writeToPortableBackend(() => setSettingRPC(key, value));
}

export function discoverLocalAIEndpoints() {
  return readFromBackend<AIEndpoint[]>([], () =>
    appBackend.DiscoverLocalAIEndpoints(),
  );
}

export async function classifyAIEndpoint(baseURL: string) {
  if (!isShietAppAvailable()) {
    const local =
      baseURL.includes("localhost") ||
      baseURL.includes("127.0.0.1") ||
      baseURL.includes(":11434") ||
      baseURL.includes(":1234");
    return {
      local,
      verdict: local
        ? "Private · on-device"
        : "Cloud · data may leave your device",
    } satisfies AIClassification;
  }

  return appBackend.ClassifyAIEndpoint(baseURL);
}

export function listAIModels(baseURL: string, apiKey: string) {
  return writeToBackend(() => appBackend.ListAIModels(baseURL, apiKey));
}

export function validateAIConfig(
  baseURL: string,
  apiKey: string,
  model: string,
) {
  return writeToBackend(() =>
    appBackend.ValidateAIConfig(baseURL, apiKey, model),
  );
}

export function saveAIConfig(baseURL: string, model: string) {
  return writeToBackend(() => appBackend.SaveAIConfig(baseURL, model));
}

export function saveAIEndpoint(baseURL: string) {
  return writeToBackend(() => appBackend.SaveAIEndpoint(baseURL));
}

export function saveAIModel(model: string) {
  return writeToBackend(() => appBackend.SaveAIModel(model));
}

export function saveAIAPIKey(apiKey: string) {
  return writeToBackend(() => appBackend.SaveAIAPIKey(apiKey));
}

export function clearAIAPIKey() {
  return writeToBackend(() => appBackend.ClearAIAPIKey());
}

export function hasAIAPIKey() {
  return readFromBackend(false, () => appBackend.HasAIAPIKey());
}

export function saveExportFile(defaultFilename: string, content: string) {
  if (!isShietAppAvailable()) {
    const blob = new Blob([content], { type: "text/csv;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = defaultFilename;
    anchor.click();
    URL.revokeObjectURL(url);
    return Promise.resolve(defaultFilename);
  }

  return writeToBackend(() =>
    appBackend.SaveExportFile(defaultFilename, content),
  );
}

export function exportPeriodCSV(periodId: number, templateKey = "matrix_csv") {
  return writeToPortableBackend(async () => {
    const render = await renderPeriodExportRPC(periodId, templateKey);
    if (render.format !== "csv" && render.format !== "tsv") {
      throw new Error(`export template ${templateKey} is not tabular`);
    }
    return saveExportFile(render.filename, render.content);
  });
}

export function exportPeriodText(
  periodId: number,
  templateKey = "text_summary",
) {
  return writeToPortableBackend(async () => {
    const render = await renderPeriodExportRPC(periodId, templateKey);
    if (render.format !== "text") {
      throw new Error(`export template ${templateKey} is not text`);
    }
    return render.content;
  });
}

export function listExportTemplates() {
  return readFromPortableBackend<ExportTemplate[]>([], listExportTemplatesRPC);
}

export function getExportTemplate(key: string) {
  return readFromPortableBackend<ExportTemplate | null>(null, () =>
    getExportTemplateRPC(key),
  );
}

export function buildPeriodExport(periodId: number) {
  return readFromPortableBackend<PeriodExportModel | null>(null, () =>
    buildPeriodExportRPC(periodId),
  );
}

export function createExportTemplate(input: CreateExportTemplateInput) {
  return writeToPortableBackend(() => createExportTemplateRPC(input));
}

export function updateExportTemplate(input: UpdateExportTemplateInput) {
  return writeToPortableBackend(() => updateExportTemplateRPC(input));
}

export function deleteExportTemplate(id: number) {
  return writeToPortableBackend(() => deleteExportTemplateRPC(id));
}

export function duplicateExportTemplate(key: string) {
  return writeToPortableBackend(() => duplicateExportTemplateRPC(key));
}

export function previewExport(input: PreviewExportInput) {
  return writeToPortableBackend(() => previewExportRPC(input));
}

export function listExportFieldCatalog(grain: string, layout: string) {
  return readFromPortableBackend<ExportFieldInfo[]>([], () => listExportFieldCatalogRPC(grain, layout));
}

export function listIntegrationConnections() {
  return readFromPortableBackend<IntegrationConnection[]>([], listIntegrationConnectionsRPC);
}

export function listIntegrationProviders() {
  return readFromPortableBackend<IntegrationProvider[]>([], listIntegrationProvidersRPC);
}

export function getIntegrationAuthStatus(provider: string) {
  return readFromPortableBackend<IntegrationAuthStatus>(
    { provider, mode: "broker", brokerBaseUrl: "", oauthAvailable: true, supportsPat: provider === "github" },
    () => getIntegrationAuthStatusRPC(provider),
  );
}

export function connectIntegration(input: ConnectIntegrationInput) {
  return writeToPortableBackend(() => connectIntegrationRPC(input));
}

export function disconnectIntegration(provider: string, accountID: string) {
  return writeToPortableBackend(() => disconnectIntegrationRPC(provider, accountID));
}

export function getLogPath() {
  return readFromBackend("", () => appBackend.LogPath());
}

export function revealLogFolder() {
  return writeToBackend(() => appBackend.RevealLogFolder());
}

export function connectGitHub(pat: string) {
  return connectIntegration({ provider: "github", pat });
}

export function githubAuthMode() {
  return getIntegrationAuthStatus("github").then((status) => status.mode);
}

export function githubOAuthAvailable() {
  return getIntegrationAuthStatus("github").then((status) => status.oauthAvailable);
}

export function disconnectGitHub(accountID: string) {
  return disconnectIntegration("github", accountID);
}

export function listGitHubRepos() {
  return readFromPortableBackend<GitHubRepo[]>([], listGitHubReposRPC);
}

export function setGitHubRepoSelected(repoID: number, selected: boolean) {
  return writeToPortableBackend(() => setGitHubRepoSelectedRPC(repoID, selected));
}

export function refreshGitHubRepos(accountID: string) {
  return writeToPortableBackend(() => refreshGitHubReposRPC(accountID));
}

export function connectSlack() {
  return connectIntegration({ provider: "slack" });
}

export function slackAuthMode() {
  return getIntegrationAuthStatus("slack").then((status) => status.mode);
}

export function slackOAuthAvailable() {
  return getIntegrationAuthStatus("slack").then((status) => status.oauthAvailable);
}

export function disconnectSlack(accountID: string) {
  return disconnectIntegration("slack", accountID);
}

export function listSlackChannels() {
  return readFromPortableBackend<SlackChannel[]>([], listSlackChannelsRPC);
}

export function setSlackChannelSelected(channelID: number, selected: boolean) {
  return writeToPortableBackend(() => setSlackChannelSelectedRPC(channelID, selected));
}

export function refreshSlackChannels(accountID: string) {
  return writeToPortableBackend(() => refreshSlackChannelsRPC(accountID));
}

export function connectBitbucket() {
  return connectIntegration({ provider: "bitbucket" });
}

export function bitbucketAuthMode() {
  return getIntegrationAuthStatus("bitbucket").then((status) => status.mode);
}

export function bitbucketOAuthAvailable() {
  return getIntegrationAuthStatus("bitbucket").then((status) => status.oauthAvailable);
}

export function disconnectBitbucket(accountID: string) {
  return disconnectIntegration("bitbucket", accountID);
}

export function listBitbucketWorkspaces() {
  return readFromPortableBackend<BitbucketWorkspace[]>([], listBitbucketWorkspacesRPC);
}

export function listBitbucketRepos() {
  return readFromPortableBackend<BitbucketRepo[]>([], listBitbucketReposRPC);
}

export function setBitbucketWorkspaceSelected(workspaceID: number, selected: boolean) {
  return writeToPortableBackend(() =>
    setBitbucketWorkspaceSelectedRPC(workspaceID, selected),
  );
}

export function setBitbucketRepoSelected(repoID: number, selected: boolean) {
  return writeToPortableBackend(() => setBitbucketRepoSelectedRPC(repoID, selected));
}

export function refreshBitbucketResources(accountID: string) {
  return writeToPortableBackend(() => refreshBitbucketResourcesRPC(accountID));
}

export function setCalendarSelected(calendarID: number, selected: boolean) {
  return writeToPortableBackend(() => setCalendarSelectedRPC(calendarID, selected));
}

export function setCalendarDefaultCategory(
  calendarID: number,
  categoryID: number | null,
) {
  return writeToPortableBackend(() => setCalendarDefaultCategoryRPC(calendarID, categoryID));
}

export function listGapEvidence(window: TimeWindow) {
  return writeToPortableBackend(() => listGapEvidenceRPC(window));
}

export function suggestGapFill(window: TimeWindow) {
  return writeToPortableBackend(() => suggestGapFillRPC(window));
}

export function syncPeriod(periodID: number) {
  return writeToPortableBackend(() => syncPeriodRPC(periodID));
}
