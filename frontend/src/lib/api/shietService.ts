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
  ExcludeEventResult,
  GapFill,
  GapSuggestion,
  GitHubRepo,
  GoogleAuthStatus,
  IntegrationConnection,
  ManualEventDeleteInput,
  ManualEventInput,
  ManualEventResult,
  ManualEventUpdateInput,
  Period,
  ReviewDecision,
  ResolveReviewDecisionInput,
  ResolveReviewDecisionResult,
  SyncResult,
  TimeWindow,
  TzSegment,
  UpdateCategoryInput,
  ExportTemplate,
  CreateExportTemplateInput,
  UpdateExportTemplateInput,
  PreviewExportInput,
  PeriodExportRender,
  ExportFieldInfo,
} from "./types";

interface ShietApp {
  ClassifyAIEndpoint(baseURL: string): Promise<AIClassification>;
  ComputeGaps(periodId: number): Promise<DayTimeline[]>;
  ConnectGitHub(pat: string): Promise<IntegrationConnection>;
  ConnectGoogle(accountID: string, accountLabel: string): Promise<IntegrationConnection>;
  CreateCategory(input: CreateCategoryInput): Promise<Category>;
  CreateGapFill(input: ManualEventInput): Promise<ManualEventResult>;
  CreateManualEvent(input: ManualEventInput): Promise<ManualEventResult>;
  DeleteManualEvent(input: ManualEventDeleteInput): Promise<ManualEventResult>;
  DeleteCategory(id: number): Promise<void>;
  DisconnectGitHub(accountID: string): Promise<void>;
  DisconnectGoogle(accountID: string): Promise<void>;
  DiscoverLocalAIEndpoints(): Promise<AIEndpoint[]>;
  EnsureCurrentPeriod(today: string, ianaTz: string): Promise<Period>;
  ExcludeEvent(input: ExcludeEventInput): Promise<ExcludeEventResult>;
  GetGoogleAuthStatus(): Promise<GoogleAuthStatus>;
  GetSetting(key: string): Promise<string>;
  GitHubAuthMode(): Promise<string>;
  GitHubOAuthAvailable(): Promise<boolean>;
  ListAIModels(baseURL: string, apiKey: string): Promise<string[]>;
  ListCalendars(): Promise<Calendar[]>;
  ListCategories(): Promise<Category[]>;
  ListEventCategoryOverlays(periodId: number): Promise<EventCategoryOverlay[]>;
  ListEvents(periodId: number): Promise<Event[]>;
  ListGapFills(periodId: number): Promise<GapFill[]>;
  ListGitHubRepos(): Promise<GitHubRepo[]>;
  ListIntegrationConnections(): Promise<IntegrationConnection[]>;
  ListReviewDecisions(periodId: number): Promise<ReviewDecision[]>;
  ResolveReviewDecision(
    input: ResolveReviewDecisionInput,
  ): Promise<ResolveReviewDecisionResult>;
  ListPeriods(): Promise<Period[]>;
  ListSelectedCalendars(): Promise<Calendar[]>;
  ListTzSegments(periodId: number): Promise<TzSegment[]>;
  RefreshGitHubRepos(accountID: string): Promise<void>;
  SaveAIConfig(baseURL: string, model: string): Promise<void>;
  SaveAIEndpoint(baseURL: string): Promise<void>;
  SaveAIModel(model: string): Promise<void>;
  SaveExportFile(defaultFilename: string, content: string): Promise<string>;
  ExportPeriodCSV(periodId: number, templateKey: string): Promise<string>;
  ExportPeriodText(periodId: number, templateKey: string): Promise<string>;
  ListExportTemplates(): Promise<ExportTemplate[]>;
  CreateExportTemplate(input: CreateExportTemplateInput): Promise<ExportTemplate>;
  UpdateExportTemplate(input: UpdateExportTemplateInput): Promise<ExportTemplate>;
  DeleteExportTemplate(id: number): Promise<void>;
  DuplicateExportTemplate(key: string): Promise<ExportTemplate>;
  PreviewExport(input: PreviewExportInput): Promise<PeriodExportRender>;
  ListExportFieldCatalog(grain: string, layout: string): Promise<ExportFieldInfo[]>;
  SetCalendarDefaultCategory(calendarID: number, categoryID: number | null): Promise<void>;
  SetCalendarSelected(calendarID: number, selected: boolean): Promise<void>;
  SetGitHubRepoSelected(repoID: number, selected: boolean): Promise<void>;
  SetSetting(key: string, value: string): Promise<void>;
  SuggestGapFill(window: TimeWindow): Promise<GapSuggestion>;
  SyncPeriod(periodID: number): Promise<SyncResult>;
  UpdateCategory(input: UpdateCategoryInput): Promise<Category>;
  UpdateManualEvent(input: ManualEventUpdateInput): Promise<ManualEventResult>;
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

async function writeToBackend<T>(write: () => Promise<T>) {
  if (!isShietAppAvailable()) {
    throw new Error("shiet backend is unavailable");
  }

  return write();
}

export function listPeriods() {
  return readFromBackend<Period[]>([], () => appBackend.ListPeriods());
}

export function ensureCurrentPeriod(today: string, ianaTz: string) {
  return readFromBackend<Period | null>(null, () =>
    appBackend.EnsureCurrentPeriod(today, ianaTz),
  );
}

export function listCategories() {
  return readFromBackend<Category[]>([], () =>
    appBackend.ListCategories(),
  );
}

export function createCategory(input: CreateCategoryInput) {
  return writeToBackend(() => appBackend.CreateCategory(input));
}

export function updateCategory(input: UpdateCategoryInput) {
  return writeToBackend(() => appBackend.UpdateCategory(input));
}

export function deleteCategory(id: number) {
  return writeToBackend(() => appBackend.DeleteCategory(id));
}

export function listEventCategoryOverlays(periodId: number) {
  return readFromBackend<EventCategoryOverlay[]>([], () =>
    appBackend.ListEventCategoryOverlays(periodId),
  );
}

export function listCalendars() {
  return readFromBackend<Calendar[]>([], () =>
    appBackend.ListCalendars(),
  );
}

export function listSelectedCalendars() {
  return readFromBackend<Calendar[]>([], () =>
    appBackend.ListSelectedCalendars(),
  );
}

export function listEvents(periodId: number) {
  return readFromBackend<Event[]>([], () =>
    appBackend.ListEvents(periodId),
  );
}

export function listGapFills(periodId: number) {
  return readFromBackend<GapFill[]>([], () =>
    appBackend.ListGapFills(periodId),
  );
}

export function createGapFill(input: ManualEventInput) {
  return writeToBackend(() => appBackend.CreateGapFill(input));
}

export function createManualEvent(input: ManualEventInput) {
  return writeToBackend(() =>
    appBackend.CreateManualEvent(input),
  );
}

export function updateManualEvent(input: ManualEventUpdateInput) {
  return writeToBackend(() =>
    appBackend.UpdateManualEvent(input),
  );
}

export function deleteManualEvent(input: ManualEventDeleteInput) {
  return writeToBackend(() =>
    appBackend.DeleteManualEvent(input),
  );
}

export function listReviewDecisions(periodId: number) {
  return readFromBackend<ReviewDecision[]>([], () =>
    appBackend.ListReviewDecisions(periodId),
  );
}

export function resolveReviewDecision(input: ResolveReviewDecisionInput) {
  return writeToBackend(() => appBackend.ResolveReviewDecision(input));
}

export function excludeEvent(input: ExcludeEventInput) {
  return writeToBackend(() => appBackend.ExcludeEvent(input));
}

export function listTzSegments(periodId: number) {
  return readFromBackend<TzSegment[]>([], () =>
    appBackend.ListTzSegments(periodId),
  );
}

export function computeGaps(periodId: number) {
  return readFromBackend<DayTimeline[]>([], () =>
    appBackend.ComputeGaps(periodId),
  );
}

export function getSetting(key: string) {
  return readFromBackend<string | null>(
    localStorage.getItem(`shiet.setting.${key}`),
    () => appBackend.GetSetting(key),
  );
}

export function setSetting(key: string, value: string) {
  if (!isShietAppAvailable()) {
    localStorage.setItem(`shiet.setting.${key}`, value);
    return Promise.resolve();
  }

  return writeToBackend(() => appBackend.SetSetting(key, value));
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
  return writeToBackend(() =>
    appBackend.ExportPeriodCSV(periodId, templateKey),
  );
}

export function exportPeriodText(
  periodId: number,
  templateKey = "text_summary",
) {
  return writeToBackend(() =>
    appBackend.ExportPeriodText(periodId, templateKey),
  );
}

export function listExportTemplates() {
  return readFromBackend<ExportTemplate[]>([], () =>
    appBackend.ListExportTemplates(),
  );
}

export function createExportTemplate(input: CreateExportTemplateInput) {
  return writeToBackend(() => appBackend.CreateExportTemplate(input));
}

export function updateExportTemplate(input: UpdateExportTemplateInput) {
  return writeToBackend(() => appBackend.UpdateExportTemplate(input));
}

export function deleteExportTemplate(id: number) {
  return writeToBackend(() => appBackend.DeleteExportTemplate(id));
}

export function duplicateExportTemplate(key: string) {
  return writeToBackend(() => appBackend.DuplicateExportTemplate(key));
}

export function previewExport(input: PreviewExportInput) {
  return writeToBackend(() => appBackend.PreviewExport(input));
}

export function listExportFieldCatalog(grain: string, layout: string) {
  return readFromBackend<ExportFieldInfo[]>([], () =>
    appBackend.ListExportFieldCatalog(grain, layout),
  );
}

export function listIntegrationConnections() {
  return readFromBackend<IntegrationConnection[]>([], () =>
    appBackend.ListIntegrationConnections(),
  );
}

export function getGoogleAuthStatus() {
  return readFromBackend<GoogleAuthStatus>(
    { mode: "broker", brokerBaseUrl: "" },
    () => appBackend.GetGoogleAuthStatus(),
  );
}

export function connectGoogle(accountID: string, accountLabel: string) {
  return writeToBackend(() =>
    appBackend.ConnectGoogle(accountID, accountLabel),
  );
}

export function disconnectGoogle(accountID: string) {
  return writeToBackend(() => appBackend.DisconnectGoogle(accountID));
}

export function connectGitHub(pat: string) {
  return writeToBackend(() => appBackend.ConnectGitHub(pat));
}

export function githubAuthMode() {
  return readFromBackend<string>("broker", () => appBackend.GitHubAuthMode());
}

export function githubOAuthAvailable() {
  return readFromBackend<boolean>(true, () => appBackend.GitHubOAuthAvailable());
}

export function disconnectGitHub(accountID: string) {
  return writeToBackend(() => appBackend.DisconnectGitHub(accountID));
}

export function listGitHubRepos() {
  return readFromBackend<GitHubRepo[]>([], () => appBackend.ListGitHubRepos());
}

export function setGitHubRepoSelected(repoID: number, selected: boolean) {
  return writeToBackend(() =>
    appBackend.SetGitHubRepoSelected(repoID, selected),
  );
}

export function refreshGitHubRepos(accountID: string) {
  return writeToBackend(() => appBackend.RefreshGitHubRepos(accountID));
}

export function setCalendarSelected(calendarID: number, selected: boolean) {
  return writeToBackend(() =>
    appBackend.SetCalendarSelected(calendarID, selected),
  );
}

export function setCalendarDefaultCategory(
  calendarID: number,
  categoryID: number | null,
) {
  return writeToBackend(() =>
    appBackend.SetCalendarDefaultCategory(calendarID, categoryID),
  );
}

export function suggestGapFill(window: TimeWindow) {
  return writeToBackend(() => appBackend.SuggestGapFill(window));
}

export function syncPeriod(periodID: number) {
  return writeToBackend(() => appBackend.SyncPeriod(periodID));
}
