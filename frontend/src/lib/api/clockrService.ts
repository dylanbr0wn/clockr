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
  GapFill,
  GapSuggestion,
  IntegrationConnection,
  ManualEventDeleteInput,
  ManualEventInput,
  ManualEventResult,
  ManualEventUpdateInput,
  Period,
  ReviewItem,
  ResolveReviewItemInput,
  ResolveReviewItemResult,
  SyncResult,
  TimeWindow,
  TzSegment,
  UpdateCategoryInput,
} from "./types";

interface ClockrApp {
  ClassifyAIEndpoint(baseURL: string): Promise<AIClassification>;
  ComputeGaps(periodId: number): Promise<DayTimeline[]>;
  ConnectGoogle(accountID: string, accountLabel: string): Promise<IntegrationConnection>;
  CreateCategory(input: CreateCategoryInput): Promise<Category>;
  CreateGapFill(input: ManualEventInput): Promise<ManualEventResult>;
  CreateManualEvent(input: ManualEventInput): Promise<ManualEventResult>;
  DeleteManualEvent(input: ManualEventDeleteInput): Promise<ManualEventResult>;
  DeleteCategory(id: number): Promise<void>;
  DisconnectGoogle(accountID: string): Promise<void>;
  DiscoverLocalAIEndpoints(): Promise<AIEndpoint[]>;
  EnsureCurrentPeriod(today: string, ianaTz: string): Promise<Period>;
  GetSetting(key: string): Promise<string>;
  ListAIModels(baseURL: string, apiKey: string): Promise<string[]>;
  ListCalendars(): Promise<Calendar[]>;
  ListCategories(): Promise<Category[]>;
  ListEvents(periodId: number): Promise<Event[]>;
  ListGapFills(periodId: number): Promise<GapFill[]>;
  ListIntegrationConnections(): Promise<IntegrationConnection[]>;
  ListOpenReviewItems(periodId: number): Promise<ReviewItem[]>;
  ResolveReviewItem(
    input: ResolveReviewItemInput,
  ): Promise<ResolveReviewItemResult>;
  ListPeriods(): Promise<Period[]>;
  ListSelectedCalendars(): Promise<Calendar[]>;
  ListTzSegments(periodId: number): Promise<TzSegment[]>;
  SaveAIConfig(baseURL: string, model: string): Promise<void>;
  SaveAIEndpoint(baseURL: string): Promise<void>;
  SaveAIModel(model: string): Promise<void>;
  SaveExportFile(defaultFilename: string, content: string): Promise<string>;
  SetCalendarDefaultCategory(calendarID: number, categoryID: number | null): Promise<void>;
  SetCalendarSelected(calendarID: number, selected: boolean): Promise<void>;
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

const appBackend = generatedApp as unknown as ClockrApp;

export function isClockrAppAvailable() {
  return Boolean(
    typeof window !== "undefined" &&
      window.go?.main?.App,
  );
}

async function readFromBackend<T>(fallback: T, read: () => Promise<T>) {
  if (!isClockrAppAvailable()) {
    return fallback;
  }

  return read();
}

async function writeToBackend<T>(write: () => Promise<T>) {
  if (!isClockrAppAvailable()) {
    throw new Error("Clockr backend is unavailable");
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

export function listOpenReviewItems(periodId: number) {
  return readFromBackend<ReviewItem[]>([], () =>
    appBackend.ListOpenReviewItems(periodId),
  );
}

export function resolveReviewItem(input: ResolveReviewItemInput) {
  return writeToBackend(() => appBackend.ResolveReviewItem(input));
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
    localStorage.getItem(`clockr.setting.${key}`),
    () => appBackend.GetSetting(key),
  );
}

export function setSetting(key: string, value: string) {
  if (!isClockrAppAvailable()) {
    localStorage.setItem(`clockr.setting.${key}`, value);
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
  if (!isClockrAppAvailable()) {
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
  if (!isClockrAppAvailable()) {
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

export function listIntegrationConnections() {
  return readFromBackend<IntegrationConnection[]>([], () =>
    appBackend.ListIntegrationConnections(),
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
