export interface Category {
  id: number;
  name: string;
  description: string;
  key: string;
  color: string;
  isDefaultGap: boolean;
}

export interface CreateCategoryInput {
  name: string;
  description?: string;
  key?: string;
  color?: string;
  isDefaultGap?: boolean;
}

export interface UpdateCategoryInput {
  id: number;
  name: string;
  description?: string;
  key?: string;
  color?: string;
  isDefaultGap?: boolean;
}

export interface EventCategoryOverlay {
  provider: string;
  externalId: string;
  instanceId?: string;
  categoryId: number;
}

export interface Calendar {
  id: number;
  provider: string;
  externalId: string;
  name: string;
  isPrimary: boolean;
  selected: boolean;
  defaultCategoryId?: number;
}

export interface IntegrationConnection {
  id: number;
  provider: string;
  accountLabel: string;
  accountId: string;
  scopes: string[];
  status: "connected" | "needs_reauth" | "disconnected" | string;
  connectedAt: string;
  updatedAt: string;
}

export interface GoogleAuthStatus {
  mode: "broker" | "local" | string;
  brokerBaseUrl: string;
}

export interface GitHubRepo {
  id: number;
  accountId: string;
  externalId: string;
  name: string;
  fullName: string;
  private: boolean;
  selected: boolean;
}

export interface SyncResult {
  added: number;
  updated: number;
  unchanged: number;
  removed: number;
  flagged: number;
}

export interface Period {
  id: number;
  startDate: string;
  endDate: string;
  cadence: string;
  anchorDate: string;
  targetHoursPerDay: number;
  lastSyncedAt?: string;
}

export interface Event {
  id: number;
  periodId: number;
  calendarId: number;
  provider: string;
  externalId: string;
  instanceId?: string;
  recurringEventId?: string;
  icalUid?: string;
  title: string;
  description?: string;
  location?: string;
  organizer?: string;
  allDay: boolean;
  start?: string;
  end?: string;
  startDate?: string;
  endDate?: string;
  originalTz?: string;
  active: boolean;
}

export interface GapFill {
  id: number;
  periodId: number;
  day: string;
  start: string;
  end: string;
  categoryId?: number;
  note?: string;
  source: string;
}

export interface ManualEventInput {
  periodId: number;
  day: string;
  startMinutes: number;
  endMinutes: number;
  categoryId?: number;
  note?: string;
}

export interface ManualEventUpdateInput extends ManualEventInput {
  id: number;
}

export interface ManualEventDeleteInput {
  id: number;
  periodId: number;
}

export interface ManualEventResult {
  periodId: number;
  id: number;
}

export interface ReviewDecisionAction {
  key: string;
  label: string;
  role: "primary" | "secondary";
  variant?: "default" | "outline" | "destructive";
}

export interface ReviewDecision {
  id: number;
  kind: string;
  eventId?: number;
  tag: string;
  title: string;
  description: string;
  actions: ReviewDecisionAction[];
}

export interface ResolveReviewDecisionInput {
  decisionId: number;
  action: string;
}

export interface ResolveReviewDecisionResult {
  periodId: number;
}

export interface ExcludeEventInput {
  eventId: number;
  periodId: number;
}

export interface ExcludeEventResult {
  periodId: number;
  eventId: number;
}

export interface TzSegment {
  id: number;
  periodId: number;
  effectiveFromDate: string;
  ianaTz: string;
}

export interface Interval {
  start: string;
  end: string;
}

export interface DayTimeline {
  date: string;
  tz: string;
  windowStart: string;
  windowEnd: string;
  events: Interval[];
  filled: Interval[];
  gaps: Interval[];
  coveredHours: number;
  gapHours: number;
}

export interface AIEndpoint {
  name: string;
  baseUrl: string;
  local: boolean;
  running: boolean;
  models?: string[];
}

export interface AIValidationResult {
  ok: boolean;
  local: boolean;
  verdict: string;
  message: string;
}

export interface AIClassification {
  local: boolean;
  verdict: string;
}

export interface GapSuggestion {
  category: string;
  description: string;
  evidenceCount: number;
}

export interface TimeWindow {
  start: string;
  end: string;
}

export interface ExportTemplate {
  id: number;
  key: string;
  name: string;
  description: string;
  format: "csv" | "tsv" | "text" | string;
  builtin: boolean;
  body: string;
}

export interface CreateExportTemplateInput {
  key?: string;
  name: string;
  description?: string;
  format: "csv" | "tsv" | "text" | string;
  body: string;
}

export interface UpdateExportTemplateInput {
  id: number;
  name: string;
  description?: string;
  format: "csv" | "tsv" | "text" | string;
  body: string;
}

export interface PreviewExportInput {
  periodId: number;
  templateKey?: string;
  format?: string;
  body?: string;
}

export interface PeriodExportRender {
  filename: string;
  content: string;
  format: string;
}

export interface ExportFieldInfo {
  field: string;
  label: string;
  description: string;
}

