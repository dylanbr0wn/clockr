import * as generatedApp from "../../../wailsjs/go/main/App";
import type {
  Calendar,
  Category,
  DayTimeline,
  Event,
  GapFill,
  ManualEventDeleteInput,
  ManualEventInput,
  ManualEventResult,
  ManualEventUpdateInput,
  Period,
  ReviewItem,
  TzSegment,
} from "./types";

interface ClockrApp {
  ComputeGaps(periodId: number): Promise<DayTimeline[]>;
  CreateManualEvent(input: ManualEventInput): Promise<ManualEventResult>;
  DeleteManualEvent(input: ManualEventDeleteInput): Promise<ManualEventResult>;
  EnsureCurrentPeriod(today: string, ianaTz: string): Promise<Period>;
  GetSetting(key: string): Promise<string>;
  ListCalendars(): Promise<Calendar[]>;
  ListCategories(): Promise<Category[]>;
  ListEvents(periodId: number): Promise<Event[]>;
  ListGapFills(periodId: number): Promise<GapFill[]>;
  ListOpenReviewItems(periodId: number): Promise<ReviewItem[]>;
  ListPeriods(): Promise<Period[]>;
  ListSelectedCalendars(): Promise<Calendar[]>;
  ListTzSegments(periodId: number): Promise<TzSegment[]>;
  UpdateManualEvent(input: ManualEventUpdateInput): Promise<ManualEventResult>;
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
  return readFromBackend<string | null>(null, () =>
    appBackend.GetSetting(key),
  );
}
