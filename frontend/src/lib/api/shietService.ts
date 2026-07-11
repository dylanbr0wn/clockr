import * as generatedApp from "../../../wailsjs/go/main/App";
import type {
  AIClassification,
  AIEndpoint,
  AIValidationResult,
  Calendar,
  Category,
  CreateCategoryInput,
  DayTimeline,
  Event,
  EventCategoryOverlay,
  ExcludeEventInput,
  GapFill,
  GitHubRepo,
  GoogleAuthStatus,
  IntegrationAuthStatus,
  IntegrationConnection,
  IntegrationProvider,
  ConnectIntegrationInput,
  ManualEventDeleteInput,
  ManualEventInput,
  ManualEventUpdateInput,
  Period,
  PeriodExportModel,
  ReviewDecision,
  ResolveReviewDecisionInput,
  SlackChannel,
  TimeWindow,
  TzSegment,
  UpdateCategoryInput,
  ExportTemplate,
  CreateExportTemplateInput,
  UpdateExportTemplateInput,
  PreviewExportInput,
  ExportFieldInfo,
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
  createGapFillRPC,
  createManualEventRPC,
  deleteCategoryRPC,
  deleteExportTemplateRPC,
  deleteManualEventRPC,
  disconnectIntegrationRPC,
  duplicateExportTemplateRPC,
  excludeEventRPC,
  getCategoryRPC,
  getEventRPC,
  getExportTemplateRPC,
  getIntegrationAuthStatusRPC,
  getSettingRPC,
  listCalendarsRPC,
  listCategoriesRPC,
  listEventCategoryOverlaysRPC,
  listEventsRPC,
  listExportFieldCatalogRPC,
  listExportTemplatesRPC,
  listGapFillsRPC,
  listGitHubReposRPC,
  listIntegrationConnectionsRPC,
  listIntegrationProvidersRPC,
  listReviewDecisionsRPC,
  listSelectedCalendarsRPC,
  listSlackChannelsRPC,
  listTzSegmentsRPC,
  previewExportRPC,
  refreshGitHubReposRPC,
  refreshSlackChannelsRPC,
  renderPeriodExportRPC,
  resolveReviewDecisionRPC,
  setCalendarDefaultCategoryRPC,
  setCalendarSelectedRPC,
  setGitHubRepoSelectedRPC,
  setSettingRPC,
  setSlackChannelSelectedRPC,
  suggestGapFillRPC,
  syncPeriodRPC,
  updateCategoryRPC,
  updateExportTemplateRPC,
  updateManualEventRPC,
} from "./applicationRpc";

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
  return isShietAppAvailable() || Boolean(import.meta.env.VITE_SHIET_RPC_BASE_URL?.trim());
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

export function listCategories() {
  return readFromPortableBackend<Category[]>([], listCategoriesRPC);
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

export function listEventCategoryOverlays(periodId: number) {
  return readFromPortableBackend<EventCategoryOverlay[]>([], () => listEventCategoryOverlaysRPC(periodId));
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

export function listGapFills(periodId: number) {
  return readFromPortableBackend<GapFill[]>([], () => listGapFillsRPC(periodId));
}

export function createGapFill(input: ManualEventInput) {
  return writeToPortableBackend(() => createGapFillRPC(input));
}

export function createManualEvent(input: ManualEventInput) {
  return writeToPortableBackend(() => createManualEventRPC(input));
}

export function updateManualEvent(input: ManualEventUpdateInput) {
  return writeToPortableBackend(() => updateManualEventRPC(input));
}

export function deleteManualEvent(input: ManualEventDeleteInput) {
  return writeToPortableBackend(() => deleteManualEventRPC(input));
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

export function getGoogleAuthStatus() {
  return getIntegrationAuthStatus("google").then((status) => ({
    mode: status.mode,
    brokerBaseUrl: status.brokerBaseUrl,
  } satisfies GoogleAuthStatus));
}

export function getLogPath() {
  return readFromBackend("", () => appBackend.LogPath());
}

export function revealLogFolder() {
  return writeToBackend(() => appBackend.RevealLogFolder());
}

export function connectGoogle(accountID: string, accountLabel: string) {
  return connectIntegration({ provider: "google", accountId: accountID, accountLabel });
}

export function disconnectGoogle(accountID: string) {
  return disconnectIntegration("google", accountID);
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

export function setCalendarSelected(calendarID: number, selected: boolean) {
  return writeToPortableBackend(() => setCalendarSelectedRPC(calendarID, selected));
}

export function setCalendarDefaultCategory(
  calendarID: number,
  categoryID: number | null,
) {
  return writeToPortableBackend(() => setCalendarDefaultCategoryRPC(calendarID, categoryID));
}

export function suggestGapFill(window: TimeWindow) {
  return writeToPortableBackend(() => suggestGapFillRPC(window));
}

export function syncPeriod(periodID: number) {
  return writeToPortableBackend(() => syncPeriodRPC(periodID));
}
