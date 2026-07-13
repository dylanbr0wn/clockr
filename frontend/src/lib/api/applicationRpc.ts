import { createClient } from "@connectrpc/connect";
import { timestampFromDate, type Timestamp } from "@bufbuild/protobuf/wkt";

import {
  CalendarService,
  CategoryService,
  ExportService,
  IntegrationService,
  IntegrationKind as WireIntegrationKind,
  ReviewActionRole,
  ReviewActionVariant,
  ScheduleService,
  SettingsService,
  type Calendar as WireCalendar,
  type BuildPeriodExportResponse as WirePeriodExportModel,
  type Category as WireCategory,
  type DayTimeline as WireDayTimeline,
  type Event as WireEvent,
  type ExportTemplate as WireExportTemplate,
  type GapFill as WireGapFill,
  type GapEvidenceItem as WireGapEvidenceItem,
  type GitHubRepo as WireGitHubRepo,
  type IntegrationConnection as WireIntegrationConnection,
  type IntegrationDescriptor as WireIntegrationDescriptor,
  type GetIntegrationAuthStatusResponse as WireIntegrationAuthStatus,
  type ReviewDecision as WireReviewDecision,
  type SlackChannel as WireSlackChannel,
} from "@/gen/shiet/app/v1/application_pb";
import type {
  Calendar,
  Category,
  CreateCategoryInput,
  CreateExportTemplateInput,
  DayTimeline,
  Event,
  EventCategoryOverlay,
  ExcludeEventInput,
  ExcludeEventResult,
  ExportFieldInfo,
  ExportTemplate,
  GapFill,
  GapEvidenceItem,
  GapSuggestion,
  GitHubRepo,
  IntegrationAuthStatus,
  IntegrationConnection,
  IntegrationKind,
  IntegrationProvider,
  ManualEventDeleteInput,
  ManualEventInput,
  ManualEventResult,
  ManualEventUpdateInput,
  PeriodExportRender,
  PeriodExportModel,
  PreviewExportInput,
  ResolveReviewDecisionInput,
  ResolveReviewDecisionResult,
  ReviewDecision,
  SlackChannel,
  SyncResult,
  TimeWindow,
  TzSegment,
  UpdateCategoryInput,
  UpdateExportTemplateInput,
} from "./types";
import { rpcTransport } from "./rpcTransport";

let category: ReturnType<typeof createClient<typeof CategoryService>> | undefined;
let calendar: ReturnType<typeof createClient<typeof CalendarService>> | undefined;
let schedule: ReturnType<typeof createClient<typeof ScheduleService>> | undefined;
let settings: ReturnType<typeof createClient<typeof SettingsService>> | undefined;
let integration: ReturnType<typeof createClient<typeof IntegrationService>> | undefined;
let exporting: ReturnType<typeof createClient<typeof ExportService>> | undefined;
const categoryClient = () => (category ??= createClient(CategoryService, rpcTransport()));
const calendarClient = () => (calendar ??= createClient(CalendarService, rpcTransport()));
const scheduleClient = () => (schedule ??= createClient(ScheduleService, rpcTransport()));
const settingsClient = () => (settings ??= createClient(SettingsService, rpcTransport()));
const integrationClient = () => (integration ??= createClient(IntegrationService, rpcTransport()));
const exportClient = () => (exporting ??= createClient(ExportService, rpcTransport()));

function safeInt(value: bigint, field: string) {
  const result = Number(value);
  if (!Number.isSafeInteger(result)) {
    throw new Error(`${field} ${value} is outside JavaScript's safe integer range`);
  }
  return result;
}

function iso(timestamp: Timestamp | undefined, field: string) {
  if (!timestamp) throw new Error(`${field} is missing`);
  return new Date(
    Number(timestamp.seconds) * 1_000 + timestamp.nanos / 1_000_000,
  ).toISOString();
}

const bigint = (value: number) => BigInt(value);

export async function listCategoriesRPC() {
  return (await categoryClient().listCategories({})).categories.map(mapCategory);
}
export async function getCategoryRPC(id: number) {
  const response = await categoryClient().getCategory({ id: bigint(id) });
  if (!response.category) throw new Error("get category response is missing category");
  return mapCategory(response.category);
}
export async function createCategoryRPC(input: CreateCategoryInput) {
  const response = await categoryClient().createCategory(input);
  if (!response.category) throw new Error("create category response is missing category");
  return mapCategory(response.category);
}
export async function updateCategoryRPC(input: UpdateCategoryInput) {
  const response = await categoryClient().updateCategory({ ...input, id: bigint(input.id) });
  if (!response.category) throw new Error("update category response is missing category");
  return mapCategory(response.category);
}
export async function deleteCategoryRPC(id: number) {
  await categoryClient().deleteCategory({ id: bigint(id) });
}
export async function listEventCategoryOverlaysRPC(periodId: number): Promise<EventCategoryOverlay[]> {
  const response = await categoryClient().listEventCategoryOverlays({ periodId: bigint(periodId) });
  return response.overlays.map((item) => ({
    provider: item.provider,
    externalId: item.externalId,
    ...(item.instanceId ? { instanceId: item.instanceId } : {}),
    categoryId: safeInt(item.categoryId, "category id"),
  }));
}

export async function listCalendarsRPC() {
  return (await calendarClient().listCalendars({})).calendars.map(mapCalendar);
}
export async function listSelectedCalendarsRPC() {
  return (await calendarClient().listSelectedCalendars({})).calendars.map(mapCalendar);
}
export async function setCalendarSelectedRPC(calendarId: number, selected: boolean) {
  await calendarClient().setCalendarSelected({ calendarId: bigint(calendarId), selected });
}
export async function setCalendarDefaultCategoryRPC(calendarId: number, categoryId: number | null) {
  await calendarClient().setCalendarDefaultCategory({
    calendarId: bigint(calendarId),
    ...(categoryId == null ? {} : { categoryId: bigint(categoryId) }),
  });
}
export async function syncPeriodRPC(periodId: number): Promise<SyncResult> {
  return calendarClient().syncPeriod({ periodId: bigint(periodId) });
}

export async function listEventsRPC(periodId: number) {
  return (await scheduleClient().listEvents({ periodId: bigint(periodId) })).events.map(mapEvent);
}
export async function getEventRPC(id: number) {
  const response = await scheduleClient().getEvent({ id: bigint(id) });
  if (!response.event) throw new Error("get event response is missing event");
  return mapEvent(response.event);
}
export async function listGapFillsRPC(periodId: number) {
  return (await scheduleClient().listGapFills({ periodId: bigint(periodId) })).gapFills.map(mapGapFill);
}
function manualInput(input: ManualEventInput) {
  return {
    periodId: bigint(input.periodId), day: input.day,
    startMinutes: input.startMinutes, endMinutes: input.endMinutes,
    ...(input.categoryId == null ? {} : { categoryId: bigint(input.categoryId) }),
    note: input.note ?? "", description: input.description ?? "",
  };
}
export async function createManualEventRPC(input: ManualEventInput): Promise<ManualEventResult> {
  const out = await scheduleClient().createManualEvent({ input: manualInput(input) });
  return { periodId: safeInt(out.periodId, "period id"), id: safeInt(out.id, "manual event id") };
}
export async function createGapFillRPC(input: ManualEventInput): Promise<ManualEventResult> {
  const out = await scheduleClient().createGapFill({ input: manualInput(input) });
  return { periodId: safeInt(out.periodId, "period id"), id: safeInt(out.id, "gap fill id") };
}
export async function updateManualEventRPC(input: ManualEventUpdateInput): Promise<ManualEventResult> {
  const out = await scheduleClient().updateManualEvent({ id: bigint(input.id), input: manualInput(input) });
  return { periodId: safeInt(out.periodId, "period id"), id: safeInt(out.id, "manual event id") };
}
export async function deleteManualEventRPC(input: ManualEventDeleteInput): Promise<ManualEventResult> {
  const out = await scheduleClient().deleteManualEvent({ id: bigint(input.id), periodId: bigint(input.periodId) });
  return { periodId: safeInt(out.periodId, "period id"), id: safeInt(out.id, "manual event id") };
}
export async function listReviewDecisionsRPC(periodId: number) {
  return (await scheduleClient().listReviewDecisions({ periodId: bigint(periodId) })).decisions.map(mapReviewDecision);
}
export async function resolveReviewDecisionRPC(input: ResolveReviewDecisionInput): Promise<ResolveReviewDecisionResult> {
  const out = await scheduleClient().resolveReviewDecision({ decisionId: bigint(input.decisionId), action: input.action });
  return { periodId: safeInt(out.periodId, "period id") };
}
export async function excludeEventRPC(input: ExcludeEventInput): Promise<ExcludeEventResult> {
  const out = await scheduleClient().excludeEvent({ eventId: bigint(input.eventId), periodId: bigint(input.periodId) });
  return { periodId: safeInt(out.periodId, "period id"), eventId: safeInt(out.eventId, "event id") };
}
export async function listTzSegmentsRPC(periodId: number): Promise<TzSegment[]> {
  const out = await scheduleClient().listTzSegments({ periodId: bigint(periodId) });
  return out.segments.map((item) => ({ id: safeInt(item.id, "timezone segment id"), periodId: safeInt(item.periodId, "period id"), effectiveFromDate: item.effectiveFromDate, ianaTz: item.ianaTz }));
}
export async function computeGapsRPC(periodId: number) {
  return (await scheduleClient().computeGaps({ periodId: bigint(periodId) })).days.map(mapDayTimeline);
}
export async function listGapEvidenceRPC(window: TimeWindow): Promise<GapEvidenceItem[]> {
  const response = await scheduleClient().listGapEvidence({
    start: timestampFromDate(new Date(window.start)),
    end: timestampFromDate(new Date(window.end)),
  });
  return response.items.map(mapGapEvidenceItem);
}

export async function suggestGapFillRPC(window: TimeWindow): Promise<GapSuggestion> {
  return scheduleClient().suggestGapFill({ start: timestampFromDate(new Date(window.start)), end: timestampFromDate(new Date(window.end)) });
}

export async function getSettingRPC(key: string) { return (await settingsClient().getSetting({ key })).value; }
export async function setSettingRPC(key: string, value: string) { await settingsClient().setSetting({ key, value }); }

export async function listIntegrationConnectionsRPC() {
  return (await integrationClient().listIntegrationConnections({})).connections.map(mapConnection);
}
export async function listIntegrationProvidersRPC(): Promise<IntegrationProvider[]> {
  return (await integrationClient().listIntegrationProviders({})).providers.map(mapIntegrationProvider);
}
export async function getIntegrationAuthStatusRPC(provider: string): Promise<IntegrationAuthStatus> {
  return mapIntegrationAuthStatus(await integrationClient().getIntegrationAuthStatus({ provider }));
}
export async function connectIntegrationRPC(input: {
  provider: string;
  accountId?: string;
  accountLabel?: string;
  pat?: string;
}): Promise<IntegrationConnection> {
  const out = await integrationClient().connectIntegration({
    provider: input.provider,
    accountId: input.accountId ?? "",
    accountLabel: input.accountLabel ?? "",
    pat: input.pat ?? "",
  });
  if (!out.connection) throw new Error("connect integration response is missing connection");
  return mapConnection(out.connection);
}
export async function disconnectIntegrationRPC(provider: string, accountId: string) {
  await integrationClient().disconnectIntegration({ provider, accountId });
}
export async function listGitHubReposRPC() { return (await integrationClient().listGitHubRepos({})).repos.map(mapGitHubRepo); }
export async function setGitHubRepoSelectedRPC(repoId: number, selected: boolean) { await integrationClient().setGitHubRepoSelected({ repoId: bigint(repoId), selected }); }
export async function refreshGitHubReposRPC(accountId: string) { await integrationClient().refreshGitHubRepos({ accountId }); }
export async function listSlackChannelsRPC() { return (await integrationClient().listSlackChannels({})).channels.map(mapSlackChannel); }
export async function setSlackChannelSelectedRPC(channelId: number, selected: boolean) { await integrationClient().setSlackChannelSelected({ channelId: bigint(channelId), selected }); }
export async function refreshSlackChannelsRPC(accountId: string) { await integrationClient().refreshSlackChannels({ accountId }); }

export async function renderPeriodExportRPC(periodId: number, templateKey: string): Promise<PeriodExportRender> {
  return exportClient().renderPeriodExport({ periodId: bigint(periodId), templateKey });
}
export async function buildPeriodExportRPC(periodId: number): Promise<PeriodExportModel> {
  return mapPeriodExportModel(await exportClient().buildPeriodExport({ periodId: bigint(periodId) }));
}
export async function listExportTemplatesRPC() { return (await exportClient().listExportTemplates({})).templates.map(mapExportTemplate); }
export async function getExportTemplateRPC(key: string) {
  const out = await exportClient().getExportTemplate({ key });
  if (!out.template) throw new Error("get export template response is missing template");
  return mapExportTemplate(out.template);
}
export async function createExportTemplateRPC(input: CreateExportTemplateInput) {
  const out = await exportClient().createExportTemplate({ key: input.key ?? "", name: input.name, description: input.description ?? "", format: input.format, body: input.body });
  if (!out.template) throw new Error("create export template response is missing template");
  return mapExportTemplate(out.template);
}
export async function updateExportTemplateRPC(input: UpdateExportTemplateInput) {
  const out = await exportClient().updateExportTemplate({ id: bigint(input.id), name: input.name, description: input.description ?? "", format: input.format, body: input.body });
  if (!out.template) throw new Error("update export template response is missing template");
  return mapExportTemplate(out.template);
}
export async function deleteExportTemplateRPC(id: number) { await exportClient().deleteExportTemplate({ id: bigint(id) }); }
export async function duplicateExportTemplateRPC(key: string) {
  const out = await exportClient().duplicateExportTemplate({ key });
  if (!out.template) throw new Error("duplicate export template response is missing template");
  return mapExportTemplate(out.template);
}
export async function previewExportRPC(input: PreviewExportInput): Promise<PeriodExportRender> {
  return exportClient().previewExport({ periodId: bigint(input.periodId), templateKey: input.templateKey ?? "", format: input.format ?? "", body: input.body ?? "" });
}
export async function listExportFieldCatalogRPC(grain: string, layout: string): Promise<ExportFieldInfo[]> {
  return (await exportClient().listExportFieldCatalog({ grain, layout })).fields;
}

export function mapCategory(item: WireCategory): Category {
  return { id: safeInt(item.id, "category id"), name: item.name, description: item.description, key: item.key, color: item.color, isDefaultGap: item.isDefaultGap };
}
function mapCalendar(item: WireCalendar): Calendar {
  return { id: safeInt(item.id, "calendar id"), provider: item.provider, externalId: item.externalId, name: item.name, isPrimary: item.isPrimary, selected: item.selected, ...(item.defaultCategoryId == null ? {} : { defaultCategoryId: safeInt(item.defaultCategoryId, "default category id") }) };
}
function mapEvent(item: WireEvent): Event {
  return { id: safeInt(item.id, "event id"), periodId: safeInt(item.periodId, "period id"), calendarId: safeInt(item.calendarId, "calendar id"), provider: item.provider, externalId: item.externalId, title: item.title, allDay: item.allDay, active: item.active,
    ...(item.instanceId ? { instanceId: item.instanceId } : {}), ...(item.recurringEventId ? { recurringEventId: item.recurringEventId } : {}), ...(item.icalUid ? { icalUid: item.icalUid } : {}), ...(item.description ? { description: item.description } : {}), ...(item.location ? { location: item.location } : {}), ...(item.organizer ? { organizer: item.organizer } : {}), ...(item.start ? { start: iso(item.start, "event start") } : {}), ...(item.end ? { end: iso(item.end, "event end") } : {}), ...(item.startDate ? { startDate: item.startDate } : {}), ...(item.endDate ? { endDate: item.endDate } : {}), ...(item.originalTz ? { originalTz: item.originalTz } : {}) };
}
function mapGapFill(item: WireGapFill): GapFill {
  return { id: safeInt(item.id, "gap fill id"), periodId: safeInt(item.periodId, "period id"), day: item.day, start: item.start, end: item.end, source: item.source, ...(item.categoryId == null ? {} : { categoryId: safeInt(item.categoryId, "category id") }), ...(item.note ? { note: item.note } : {}), ...(item.description ? { description: item.description } : {}) };
}
function mapGapEvidenceItem(item: WireGapEvidenceItem): GapEvidenceItem {
  return {
    provider: item.provider,
    kind: item.kind,
    summary: item.summary,
    source: item.source,
  };
}
export function mapReviewDecision(item: WireReviewDecision): ReviewDecision {
  return { id: safeInt(item.id, "review decision id"), kind: item.kind, tag: item.tag, title: item.title, description: item.description, ...(item.eventId == null ? {} : { eventId: safeInt(item.eventId, "event id") }), actions: item.actions.map((action) => ({ key: action.key, label: action.label, role: mapReviewRole(action.role), ...mapReviewVariant(action.variant) })) };
}
function mapReviewRole(role: ReviewActionRole): "primary" | "secondary" {
  if (role === ReviewActionRole.PRIMARY) return "primary";
  if (role === ReviewActionRole.SECONDARY) return "secondary";
  throw new Error(`unknown review action role ${role}`);
}
function mapReviewVariant(variant: ReviewActionVariant): { variant?: "default" | "outline" | "destructive" } {
  switch (variant) {
    case ReviewActionVariant.UNSPECIFIED: return {};
    case ReviewActionVariant.DEFAULT: return { variant: "default" };
    case ReviewActionVariant.OUTLINE: return { variant: "outline" };
    case ReviewActionVariant.DESTRUCTIVE: return { variant: "destructive" };
    default: throw new Error(`unknown review action variant ${variant}`);
  }
}
function mapDayTimeline(item: WireDayTimeline): DayTimeline {
  const intervals = (values: WireDayTimeline["events"]) => values.map((value) => ({ start: iso(value.start, "interval start"), end: iso(value.end, "interval end") }));
  return { date: item.date, tz: item.tz, windowStart: iso(item.windowStart, "window start"), windowEnd: iso(item.windowEnd, "window end"), events: intervals(item.events), filled: intervals(item.filled), gaps: intervals(item.gaps), coveredHours: item.coveredHours, gapHours: item.gapHours };
}
function mapConnection(item: WireIntegrationConnection): IntegrationConnection { return { id: safeInt(item.id, "connection id"), provider: item.provider, accountLabel: item.accountLabel, accountId: item.accountId, scopes: item.scopes, status: item.status, connectedAt: item.connectedAt, updatedAt: item.updatedAt }; }
function mapIntegrationKind(kind: WireIntegrationKind): IntegrationKind {
  if (kind === WireIntegrationKind.CALENDAR_SOURCE) return "calendar_source";
  if (kind === WireIntegrationKind.ACTIVITY_EVIDENCE) return "activity_evidence";
  throw new Error(`unknown integration kind ${kind}`);
}
function mapIntegrationProvider(item: WireIntegrationDescriptor): IntegrationProvider {
  if (!item.connect) throw new Error("integration descriptor is missing connect capabilities");
  return {
    id: item.id,
    displayName: item.displayName,
    kind: mapIntegrationKind(item.kind),
    connect: {
      needsAccountHint: item.connect.needsAccountHint,
      supportsPat: item.connect.supportsPat,
      oauthAvailable: item.connect.oauthAvailable,
    },
  };
}
function mapIntegrationAuthStatus(item: WireIntegrationAuthStatus): IntegrationAuthStatus {
  return {
    provider: item.provider,
    mode: item.mode,
    brokerBaseUrl: item.brokerBaseUrl,
    oauthAvailable: item.oauthAvailable,
    supportsPat: item.supportsPat,
  };
}
function mapGitHubRepo(item: WireGitHubRepo): GitHubRepo { return { id: safeInt(item.id, "repository id"), accountId: item.accountId, externalId: item.externalId, name: item.name, fullName: item.fullName, private: item.private, selected: item.selected }; }
function mapSlackChannel(item: WireSlackChannel): SlackChannel { return { id: safeInt(item.id, "channel id"), accountId: item.accountId, externalId: item.externalId, name: item.name, private: item.private, selected: item.selected }; }
function mapExportTemplate(item: WireExportTemplate): ExportTemplate { return { id: safeInt(item.id, "export template id"), key: item.key, name: item.name, description: item.description, format: item.format, builtin: item.builtin, body: item.body }; }
export function mapPeriodExportModel(item: WirePeriodExportModel): PeriodExportModel {
  const category = (value: WirePeriodExportModel["periodTotals"][number]["category"]) => value ? ({ ...(value.id == null ? {} : { id: safeInt(value.id, "export category id") }), name: value.name, key: value.key, ...(value.color ? { color: value.color } : {}) }) : ({ name: "", key: "" });
  const totals = (values: WirePeriodExportModel["periodTotals"]) => values.map((value) => ({ category: category(value.category), minutes: value.minutes }));
  return { periodId: safeInt(item.periodId, "period id"), periodLabel: item.periodLabel, startDate: item.startDate, endDate: item.endDate, targetHoursPerDay: item.targetHoursPerDay, targetMinutes: item.targetMinutes, actualMinutes: item.actualMinutes, days: item.days,
    entries: item.entries.map((entry) => ({ source: entry.source, sourceId: safeInt(entry.sourceId, "export source id"), day: entry.day, startMinutes: entry.startMinutes, endMinutes: entry.endMinutes, minutes: entry.minutes, title: entry.title, category: category(entry.category) })),
    dailyTotals: item.dailyTotals.map((day) => ({ date: day.date, categories: totals(day.categories), actualMinutes: day.actualMinutes, targetMinutes: day.targetMinutes })), periodTotals: totals(item.periodTotals) };
}
