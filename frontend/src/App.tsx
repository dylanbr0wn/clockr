import { useEffect, useMemo, useRef, useState } from "react";
import "@/index.css";
import { Button } from "@/components/ui/button";
import {
  CREATE_PREVIEW_ITEM_ID,
  Scheduler,
  SchedulerItemLayer,
  formatMinutes,
  type SchedulerChange,
  type SchedulerCreateRequest,
  type SchedulerDay,
  type SchedulerItem,
} from "@/lib/scheduler";
import {
  useCategories,
  useCreateManualEvent,
  useCurrentPeriod,
  useEvents,
  useGapFills,
  useOpenReviewItems,
  usePeriods,
  useTzSegments,
  useUpdateManualEvent,
  type Category,
  type Event as ClockrEvent,
  type GapFill,
  type Period,
  type TzSegment,
} from "@/lib/api";
import { Clock } from "lucide-react";
import { Separator } from "./components/ui/separator";
import { Environment } from "../wailsjs/runtime/runtime";
import { Card, CardContent, CardHeader, CardTitle } from "./components/ui/card";
import { cn } from "./lib/utils";

type ScheduleKind = "calendar" | "gap" | "manual" | "review";

interface ScheduleMetadata {
  title: string;
  category: string;
  kind: ScheduleKind;
}

interface ScheduleDayMetadata {
  isWeekend: boolean;
}

type ScheduleItem = SchedulerItem<ScheduleMetadata>;
type ScheduleDay = SchedulerDay<ScheduleDayMetadata>;
type SchedulePlacement = Pick<
  ScheduleItem,
  "day" | "endMinutes" | "startMinutes"
>;

const START_DATE = "2026-06-08";
const FALLBACK_DAY_COUNT = 7;
const SCHEDULE_START_MINUTES = 0;
const SCHEDULE_END_MINUTES = 24 * 60;
const WORKING_START_MINUTES = 8 * 60;
const WORKING_END_MINUTES = 18 * 60;
const INITIAL_SCROLL_CONTEXT_MINUTES = 2 * 60;
const TIMELINE_HOUR_HEIGHT = 56;
const MAC_TITLEBAR_PADDING_CLASS = "pl-24";
const DEFAULT_TITLEBAR_PADDING_CLASS = "pl-3";

function buildDays(startDate: string, count: number): ScheduleDay[] {
  const [year, month, day] = startDate.split("-").map(Number);
  const start = new Date(Date.UTC(year, month - 1, day));

  return Array.from({ length: count }, (_, index) => {
    const date = new Date(start);
    date.setUTCDate(start.getUTCDate() + index);
    const isoDate = date.toISOString().slice(0, 10);
    const dayOfWeek = date.getUTCDay();

    return {
      date: isoDate,
      label: date.toLocaleDateString(undefined, {
        weekday: "short",
        month: "short",
        day: "numeric",
        timeZone: "UTC",
      }),
      metadata: {
        isWeekend: dayOfWeek === 0 || dayOfWeek === 6,
      },
    };
  });
}

function dateFromDateKey(value: string) {
  const [year, month, day] = value.split("-").map(Number);

  if (!year || !month || !day) {
    return null;
  }

  const date = new Date(Date.UTC(year, month - 1, day));
  return Number.isNaN(date.getTime()) ? null : date;
}

function inclusiveDayCount(startDate: string, endDate: string) {
  const start = dateFromDateKey(startDate);
  const end = dateFromDateKey(endDate);

  if (!start || !end) {
    return 1;
  }

  const durationMs = end.getTime() - start.getTime();
  return Math.max(1, Math.floor(durationMs / 86_400_000) + 1);
}

function periodDayCount(period: Period | null) {
  if (!period) {
    return FALLBACK_DAY_COUNT;
  }

  return inclusiveDayCount(period.startDate, period.endDate);
}

function formatDateKey(value: string) {
  const date = dateFromDateKey(value);

  if (!date) {
    return value;
  }

  return date.toLocaleDateString(undefined, {
    month: "short",
    day: "numeric",
    timeZone: "UTC",
  });
}

function formatPeriodLabel(period: Period) {
  return `${formatDateKey(period.startDate)}-${formatDateKey(period.endDate)}`;
}

function formatCadence(value: string) {
  return value
    .split(/[-_\s]+/)
    .filter(Boolean)
    .map((part) => part.slice(0, 1).toUpperCase() + part.slice(1))
    .join(" ");
}

function kindClasses(kind: ScheduleKind) {
  switch (kind) {
    case "calendar":
      return "border-sky-300 bg-sky-50 text-sky-950";
    case "gap":
      return "border-emerald-300 bg-emerald-50 text-emerald-950";
    case "manual":
      return "border-amber-300 bg-amber-50 text-amber-950";
    case "review":
      return "border-rose-300 bg-rose-50 text-rose-950";
  }
}

function formatDuration(totalMinutes: number) {
  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;

  if (hours === 0) {
    return `${minutes}m`;
  }

  if (minutes === 0) {
    return `${hours}h`;
  }

  return `${hours}h ${minutes}m`;
}

function durationLabel(item: ScheduleItem) {
  return formatDuration(item.endMinutes - item.startMinutes);
}

function toDate(value: string | undefined) {
  if (!value) {
    return null;
  }

  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? null : date;
}

function localDateKey(date = new Date()) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function defaultTimeZone() {
  return Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
}

function zonedDateTimeParts(date: Date, timeZone: string) {
  const parts = new Intl.DateTimeFormat("en-CA", {
    day: "2-digit",
    hour: "2-digit",
    hourCycle: "h23",
    minute: "2-digit",
    month: "2-digit",
    timeZone,
    year: "numeric",
  }).formatToParts(date);
  const values = Object.fromEntries(
    parts
      .filter((part) => part.type !== "literal")
      .map((part) => [part.type, part.value]),
  );

  return {
    day: `${values.year}-${values.month}-${values.day}`,
    minutes: Number(values.hour) * 60 + Number(values.minute),
  };
}

function activeTimeZoneForDay(day: string, tzSegments: TzSegment[]) {
  if (tzSegments.length === 0) {
    return defaultTimeZone();
  }

  let activeSegment = tzSegments[0];
  for (const segment of tzSegments) {
    if (segment.effectiveFromDate <= day) {
      activeSegment = segment;
    } else {
      break;
    }
  }

  return activeSegment.ianaTz;
}

function zonedPosition(value: string | undefined, tzSegments: TzSegment[]) {
  const date = toDate(value);

  if (!date) {
    return null;
  }

  const initialTimeZone = tzSegments[0]?.ianaTz ?? defaultTimeZone();
  const initialParts = zonedDateTimeParts(date, initialTimeZone);
  const activeTimeZone = activeTimeZoneForDay(initialParts.day, tzSegments);

  if (activeTimeZone === initialTimeZone) {
    return initialParts;
  }

  return zonedDateTimeParts(date, activeTimeZone);
}

function categoryName(
  categoryId: number | undefined,
  categoriesById: Map<number, Category>,
) {
  if (typeof categoryId !== "number") {
    return "Unassigned";
  }

  return categoriesById.get(categoryId)?.name ?? "Unassigned";
}

function periodContainsDate(period: Period, day: string) {
  return period.startDate <= day && day <= period.endDate;
}

function applyPlacement(item: ScheduleItem, placement?: SchedulePlacement) {
  if (!placement) {
    return item;
  }

  return {
    ...item,
    ...placement,
  };
}

function eventToSchedulerItem(
  event: ClockrEvent,
  tzSegments: TzSegment[],
  placement?: SchedulePlacement,
): ScheduleItem | null {
  if (event.allDay && event.startDate) {
    return applyPlacement(
      {
        id: `event-${event.id}`,
        day: event.startDate,
        startMinutes: SCHEDULE_START_MINUTES,
        endMinutes: SCHEDULE_END_MINUTES,
        metadata: {
          title: event.title || "Untitled event",
          category: "Calendar",
          kind: "calendar",
        },
      },
      placement,
    );
  }

  const start = zonedPosition(event.start, tzSegments);
  const end = zonedPosition(event.end, tzSegments);

  if (!start || !end) {
    return null;
  }

  const endMinutes =
    end.day === start.day ? end.minutes : SCHEDULE_END_MINUTES;

  return applyPlacement(
    {
      id: `event-${event.id}`,
      day: start.day,
      startMinutes: start.minutes,
      endMinutes: Math.max(start.minutes + 15, endMinutes),
      metadata: {
        title: event.title || "Untitled event",
        category: "Calendar",
        kind: "calendar",
      },
    },
    placement,
  );
}

function gapFillToSchedulerItem(
  gapFill: GapFill,
  categoriesById: Map<number, Category>,
  tzSegments: TzSegment[],
  placement?: SchedulePlacement,
): ScheduleItem | null {
  const timeZone = activeTimeZoneForDay(gapFill.day, tzSegments);
  const startDate = toDate(gapFill.start);
  const endDate = toDate(gapFill.end);

  if (!startDate || !endDate) {
    return null;
  }

  const start = zonedDateTimeParts(startDate, timeZone);
  const end = zonedDateTimeParts(endDate, timeZone);
  const startMinutes = start.minutes;
  const endMinutes =
    end.day === start.day ? end.minutes : SCHEDULE_END_MINUTES;
  const category = categoryName(gapFill.categoryId, categoriesById);

  return applyPlacement(
    {
      id: `gap-fill-${gapFill.id}`,
      day: gapFill.day || start.day,
      startMinutes,
      endMinutes: Math.max(startMinutes + 15, endMinutes),
      metadata: {
        title: gapFill.note || category,
        category,
        kind: "manual",
      },
    },
    placement,
  );
}

function errorMessage(error: unknown) {
  if (error instanceof Error) {
    return error.message;
  }

  return String(error);
}

function getInitialPlatform() {
  return navigator.platform.toLowerCase().includes("mac") ? "darwin" : "";
}

function App() {
  const [selectedPeriodId, setSelectedPeriodId] = useState<number | null>(null);
  const [draftPlacements, setDraftPlacements] = useState<
    Record<string, SchedulePlacement>
  >({});
  const [preview, setPreview] = useState<SchedulerChange<ScheduleMetadata> | null>(
    null,
  );
  const [platform, setPlatform] = useState(getInitialPlatform);
  const schedulerViewportRef = useRef<HTMLDivElement | null>(null);
  const didSetInitialScrollRef = useRef(false);
  const today = useMemo(() => localDateKey(), []);
  const currentTimeZone = useMemo(() => defaultTimeZone(), []);
  const periodsQuery = usePeriods();
  const currentPeriodQuery = useCurrentPeriod(today, currentTimeZone);
  const categoriesQuery = useCategories();
  const createManualEventMutation = useCreateManualEvent();
  const updateManualEventMutation = useUpdateManualEvent();
  const persistedPeriods = useMemo(
    () => periodsQuery.data ?? [],
    [periodsQuery.data],
  );
  const currentPeriod = currentPeriodQuery.data ?? null;
  const periods = useMemo(() => {
    if (
      currentPeriod &&
      !persistedPeriods.some((period) => period.id === currentPeriod.id)
    ) {
      return [currentPeriod, ...persistedPeriods];
    }

    return persistedPeriods;
  }, [currentPeriod, persistedPeriods]);
  const categories = useMemo(
    () => categoriesQuery.data ?? [],
    [categoriesQuery.data],
  );
  const activePeriod = useMemo(
    () =>
      periods.find((period) => period.id === selectedPeriodId) ??
      currentPeriod ??
      periods.find((period) => periodContainsDate(period, today)) ??
      periods[0] ??
      null,
    [currentPeriod, periods, selectedPeriodId, today],
  );
  const activePeriodId = activePeriod?.id;
  const eventsQuery = useEvents(activePeriodId);
  const gapFillsQuery = useGapFills(activePeriodId);
  const reviewItemsQuery = useOpenReviewItems(activePeriodId);
  const tzSegmentsQuery = useTzSegments(activePeriodId);
  const categoriesById = useMemo(() => {
    return new Map(categories.map((category) => [category.id, category]));
  }, [categories]);
  const visibleDayCount = periodDayCount(activePeriod);
  const days = useMemo(
    () => buildDays(activePeriod?.startDate ?? START_DATE, visibleDayCount),
    [activePeriod?.startDate, visibleDayCount],
  );
  const gapFillsByItemId = useMemo(() => {
    return new Map(
      (gapFillsQuery.data ?? []).map((gapFill) => [
        `gap-fill-${gapFill.id}`,
        gapFill,
      ]),
    );
  }, [gapFillsQuery.data]);
  const titlebarPaddingClass =
    platform === "darwin"
      ? MAC_TITLEBAR_PADDING_CLASS
      : DEFAULT_TITLEBAR_PADDING_CLASS;
  const backendItems = useMemo(() => {
    const events = eventsQuery.data ?? [];
    const gapFills = gapFillsQuery.data ?? [];
    const tzSegments = tzSegmentsQuery.data ?? [];

    return [
      ...events
        .map((event) =>
          eventToSchedulerItem(
            event,
            tzSegments,
            draftPlacements[`event-${event.id}`],
          ),
        )
        .filter((item): item is ScheduleItem => item !== null),
      ...gapFills
        .map((gapFill) =>
          gapFillToSchedulerItem(
            gapFill,
            categoriesById,
            tzSegments,
            draftPlacements[`gap-fill-${gapFill.id}`],
          ),
        )
        .filter((item): item is ScheduleItem => item !== null),
    ];
  }, [
    categoriesById,
    draftPlacements,
    eventsQuery.data,
    gapFillsQuery.data,
    tzSegmentsQuery.data,
  ]);
  const items = backendItems;
  const totals = useMemo(() => {
    return items.reduce<Record<string, number>>((next, item) => {
      const key = item.metadata?.category ?? "Unassigned";
      next[key] = (next[key] ?? 0) + item.endMinutes - item.startMinutes;
      return next;
    }, {});
  }, [items]);
  const isBackendLoading =
    periodsQuery.isLoading ||
    currentPeriodQuery.isLoading ||
    categoriesQuery.isLoading ||
    eventsQuery.isLoading ||
    gapFillsQuery.isLoading ||
    reviewItemsQuery.isLoading ||
    tzSegmentsQuery.isLoading ||
    createManualEventMutation.isPending ||
    updateManualEventMutation.isPending;
  const backendError =
    periodsQuery.error ??
    currentPeriodQuery.error ??
    categoriesQuery.error ??
    eventsQuery.error ??
    gapFillsQuery.error ??
    reviewItemsQuery.error ??
    tzSegmentsQuery.error ??
    createManualEventMutation.error ??
    updateManualEventMutation.error;

  useEffect(() => {
    setSelectedPeriodId((current) => {
      if (
        currentPeriod &&
        (!current || !periods.some((period) => period.id === current))
      ) {
        return currentPeriod.id;
      }

      if (current && periods.some((period) => period.id === current)) {
        return current;
      }

      return currentPeriod?.id ?? periods[0]?.id ?? null;
    });
  }, [currentPeriod, periods]);

  useEffect(() => {
    setDraftPlacements({});
    setPreview(null);
  }, [activePeriodId]);

  const handleCreate = (request: SchedulerCreateRequest) => {
    if (activePeriodId) {
      createManualEventMutation.mutate({
        periodId: activePeriodId,
        day: request.day,
        startMinutes: request.startMinutes,
        endMinutes: request.endMinutes,
        note: "New block",
      });
      return;
    }
  };

  const handleCommit = (change: SchedulerChange<ScheduleMetadata>) => {
    if (change.itemId.startsWith("gap-fill-")) {
      const gapFill = gapFillsByItemId.get(change.itemId);

      if (gapFill) {
        setDraftPlacements((current) => ({
          ...current,
          [change.itemId]: {
            day: change.day,
            startMinutes: change.startMinutes,
            endMinutes: change.endMinutes,
          },
        }));
        updateManualEventMutation.mutate(
          {
            id: gapFill.id,
            periodId: gapFill.periodId,
            day: change.day,
            startMinutes: change.startMinutes,
            endMinutes: change.endMinutes,
            categoryId: gapFill.categoryId,
            note: gapFill.note ?? "",
          },
          {
            onSettled: () => {
              setDraftPlacements((current) => {
                const next = { ...current };
                delete next[change.itemId];
                return next;
              });
            },
          },
        );
      }

      setPreview(null);
      return;
    }

    if (change.itemId.startsWith("event-")) {
      setDraftPlacements((current) => ({
        ...current,
        [change.itemId]: {
          day: change.day,
          startMinutes: change.startMinutes,
          endMinutes: change.endMinutes,
        },
      }));
    }

    setPreview(null);
  };

  useEffect(() => {
    const viewport = schedulerViewportRef.current;
    if (!viewport || didSetInitialScrollRef.current) {
      return;
    }

    const visibleDuration = SCHEDULE_END_MINUTES - SCHEDULE_START_MINUTES;
    const initialMinute = Math.max(
      SCHEDULE_START_MINUTES,
      WORKING_START_MINUTES - INITIAL_SCROLL_CONTEXT_MINUTES,
    );
    const timelineHeight = Math.max(
      (visibleDuration / 60) * TIMELINE_HOUR_HEIGHT,
      760,
    );

    viewport.scrollTop =
      ((initialMinute - SCHEDULE_START_MINUTES) / visibleDuration) *
      timelineHeight;
    didSetInitialScrollRef.current = true;
  }, []);

  useEffect(() => {
    let isMounted = true;

    const loadEnvironment = async () => {
      try {
        const environment = await Environment();
        if (isMounted) {
          setPlatform(environment.platform);
        }
      } catch {
        // The Wails runtime is not present when rendering in plain Vite.
      }
    };

    void loadEnvironment();

    return () => {
      isMounted = false;
    };
  }, []);

  return (
    <main className="app-drag-region app-window relative h-screen overflow-hidden overscroll-none bg-background text-foreground">
      <div className="mx-auto flex h-full min-h-0 w-full flex-col">
        <header
          className={`shrink-0 flex items-center gap-3 py-2 pr-3 ${titlebarPaddingClass}`}
        >
          <div className="bg-primary rounded-md text-accent p-1.5">
            <Clock className="size-4" />
          </div>
          <div>
            <h1 className="text-base font-medium">Clockr</h1>
          </div>
          <Separator orientation="vertical" className="my-2" />
          <div className="grow" />
          <div className="app-no-drag flex flex-wrap items-center gap-2">
            {periods.length > 0 && (
              <select
                value={activePeriod?.id ?? ""}
                onChange={(event) =>
                  setSelectedPeriodId(Number(event.target.value))
                }
                aria-label="Period"
                className="h-8 min-w-48 rounded-lg border border-border bg-white px-2.5 text-sm font-medium text-foreground outline-none transition-colors focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
              >
                {periods.map((period) => (
                  <option key={period.id} value={period.id}>
                    {formatPeriodLabel(period)}
                  </option>
                ))}
              </select>
            )}
            <Button
              type="button"
              variant="outline"
              className="bg-white"
              disabled={
                !activePeriodId ||
                createManualEventMutation.isPending ||
                days.length === 0
              }
              onClick={() =>
                handleCreate({
                  day: days[0].date,
                  startMinutes: 11 * 60,
                  endMinutes: 12 * 60,
                })
              }
            >
              {createManualEventMutation.isPending ? "Saving" : "Block"}
            </Button>
          </div>
        </header>
        <Separator />
        <section className="grid min-h-0 flex-1 gap-4 lg:grid-cols-[minmax(0,1fr)_320px] p-3 bg-muted">
          <Scheduler
            days={days}
            items={items}
            config={{
              maxDays: visibleDayCount,
              scheduleStartMinutes: SCHEDULE_START_MINUTES,
              scheduleEndMinutes: SCHEDULE_END_MINUTES,
              workingStartMinutes: WORKING_START_MINUTES,
              workingEndMinutes: WORKING_END_MINUTES,
            }}
            onCreate={handleCreate}
            onPreviewChange={setPreview}
            onCommitChange={handleCommit}
          >
            {(scheduler) => {
              const visibleDuration =
                scheduler.visibleRange.endMinutes -
                scheduler.visibleRange.startMinutes;
              const slotPercent =
                (60 / visibleDuration) * 100;
              const timelineHeight = Math.max(
                (visibleDuration / 60) * TIMELINE_HOUR_HEIGHT,
                760,
              );
              const minuteToPercent = (minute: number) =>
                Math.min(
                  Math.max(
                    ((minute - scheduler.visibleRange.startMinutes) /
                      visibleDuration) *
                      100,
                    0,
                  ),
                  100,
                );
              const workingStartPercent = minuteToPercent(
                scheduler.config.workingStartMinutes,
              );
              const workingEndPercent = minuteToPercent(
                scheduler.config.workingEndMinutes,
              );
              const hourGridBackground = `repeating-linear-gradient(to bottom, transparent 0, transparent calc(${slotPercent}% - 1px), rgb(228 228 231) calc(${slotPercent}% - 1px), rgb(228 228 231) ${slotPercent}%)`;
              const nonWorkingHoursBackground =
                "repeating-linear-gradient(135deg, rgba(244, 244, 245, 0.7) 0 6px, transparent 6px 12px), rgba(244, 244, 245, 0.45)";
              const timelineMarks: number[] = [];

              for (
                let minute = scheduler.visibleRange.startMinutes;
                minute <= scheduler.visibleRange.endMinutes;
                minute += 60
              ) {
                timelineMarks.push(minute);
              }

              const timeLabelClass = (minute: number) => {
                if (minute === scheduler.visibleRange.startMinutes) {
                  return "absolute right-3 translate-y-0 text-xs font-medium text-zinc-500";
                }

                if (minute === scheduler.visibleRange.endMinutes) {
                  return "absolute right-3 -translate-y-full text-xs font-medium text-zinc-500";
                }

                return "absolute right-3 -translate-y-2 text-xs font-medium text-zinc-500";
              };

              return (
                <div
                  {...scheduler.getRootProps({
                    ref: schedulerViewportRef,
                    className:
                      "app-no-drag h-full min-h-0 overflow-auto overscroll-none rounded-xl bg-card text-sm text-card-foreground ring-1 ring-foreground/10",
                  })}
                >
                  <div
                    className="grid"
                    style={{
                      minWidth: `${72 + scheduler.days.length * 116}px`,
                      gridTemplateColumns: `72px repeat(${scheduler.days.length}, minmax(116px, 1fr))`,
                      gridTemplateRows: `52px ${timelineHeight}px`,
                    }}
                  >
                    <div className="sticky left-0 top-0 z-40 border-b border-r border-border bg-background" />
                    {scheduler.days.map((day, index) => (
                      <div
                        key={day.date}
                        className={cn([
                          "sticky top-0 z-30 flex items-center border-b border-border px-3",
                          day.metadata?.isWeekend ? "bg-muted" : "bg-background",
                          index % visibleDayCount !== visibleDayCount - 1
                            ? "border-r"
                            : "",
                        ])}
                      >
                        <div>
                          <p className="text-sm font-semibold text-foreground">
                            {day.label}
                          </p>
                          <p className="text-xs text-muted-foreground">{day.date}</p>
                        </div>
                      </div>
                    ))}

                    <div
                      className="sticky left-0 z-20 border-r border-border bg-background"
                    >
                      {timelineMarks.map((minute) => (
                        <div
                          key={minute}
                          className={timeLabelClass(minute)}
                          style={{
                            top: `${minuteToPercent(minute)}%`,
                          }}
                        >
                          {formatMinutes(minute)}
                        </div>
                      ))}
                    </div>

                    {scheduler.days.map((day) => {
                      const isWeekend = day.metadata?.isWeekend;

                      return (
                        <div
                          key={day.date}
                          {...scheduler.getDayColumnProps(day, {
                            className: cn([
                              "relative not-last:border-r border-border",
                              isWeekend ? "bg-muted" : "bg-background",
                            ]),
                            style: {
                              height: `${timelineHeight}px`,
                              backgroundImage: hourGridBackground,
                            },
                          })}
                        >
                          {!isWeekend && (
                            <>
                              <div
                                className="pointer-events-none absolute inset-x-0 top-0 z-0"
                                style={{
                                  height: `${workingStartPercent}%`,
                                  background: nonWorkingHoursBackground,
                                }}
                              />
                              <div
                                className="pointer-events-none absolute inset-x-0 bottom-0 z-0"
                                style={{
                                  top: `${workingEndPercent}%`,
                                  background: nonWorkingHoursBackground,
                                }}
                              />
                              {[workingStartPercent, workingEndPercent].map(
                                (percent) => (
                                  <div
                                    key={percent}
                                    className="pointer-events-none absolute inset-x-0 z-1 border-t border-border"
                                    style={{ top: `${percent}%` }}
                                  />
                                ),
                              )}
                            </>
                          )}
                          <SchedulerItemLayer scheduler={scheduler} day={day}>
                            {(layoutItem) => {
                              const item = layoutItem.item;

                              if (item.id === CREATE_PREVIEW_ITEM_ID) {
                                return (
                                  <div
                                    key={item.id}
                                    {...scheduler.getItemProps(layoutItem, {
                                      className:
                                        "pointer-events-none select-none z-20 flex flex-col justify-center rounded-md border border-dashed border-amber-300 bg-amber-50/80 px-2 py-1 text-xs text-amber-950",
                                    })}
                                  >
                                    {formatMinutes(item.startMinutes)}-
                                    {formatMinutes(item.endMinutes)}
                                  </div>
                                );
                              }

                              const metadata = item.metadata;
                              const itemClass = metadata
                                ? kindClasses(metadata.kind)
                                : "border-zinc-300 bg-zinc-50 text-zinc-950";

                              return (
                                <div
                                  key={item.id}
                                  {...scheduler.getItemProps(layoutItem, {
                                    onClick: () => {
                                      console.log("Clicked item", item);
                                    },
                                    className: [
                                      "group z-10 flex min-h-10 cursor-grab flex-col overflow-hidden rounded-md border px-2 py-1 text-left text-xs shadow-sm transition-shadow active:cursor-grabbing",
                                      layoutItem.isPreview
                                        ? "opacity-70 ring-2 ring-zinc-900/20"
                                        : "hover:shadow-md",
                                      itemClass,
                                    ].join(" "),
                                  })}
                                >
                                  <div
                                    {...scheduler.getResizeHandleProps(
                                      layoutItem,
                                      "start",
                                      {
                                        className:
                                          "absolute inset-x-2 top-0 h-2 cursor-ns-resize rounded-full opacity-0 group-hover:opacity-100",
                                      },
                                    )}
                                  />
                                  <div className="min-w-0">
                                    <p className="truncate font-semibold">
                                      {metadata?.title ?? "Untitled"}
                                    </p>
                                    <p className="truncate text-[11px] opacity-75">
                                      {formatMinutes(item.startMinutes)}-
                                      {formatMinutes(item.endMinutes)} ·{" "}
                                      {durationLabel(item)}
                                    </p>
                                  </div>
                                  <div className="mt-auto truncate text-[11px] font-medium opacity-80">
                                    {metadata?.category ?? "Unassigned"}
                                  </div>
                                  <div
                                    {...scheduler.getResizeHandleProps(
                                      layoutItem,
                                      "end",
                                      {
                                        className:
                                          "absolute inset-x-2 bottom-0 h-2 cursor-ns-resize rounded-full opacity-0 group-hover:opacity-100",
                                      },
                                    )}
                                  />
                                </div>
                              );
                            }}
                          </SchedulerItemLayer>
                        </div>
                      );
                    })}
                  </div>
                </div>
              );
            }}
          </Scheduler>
          <div className="flex flex-col gap-4">
            <Card className="app-no-drag min-h-0 space-y-4 overflow-auto overscroll-none">
              <CardHeader>
                <CardTitle className="text-sm">Totals by category</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="mt-3 space-y-2">
                  {Object.entries(totals).map(([category, minutes]) => (
                    <div
                      key={category}
                      className="flex items-center justify-between gap-3 text-sm"
                    >
                      <span className="truncate text-zinc-600">{category}</span>
                      <span className="font-semibold text-zinc-950">
                        {formatDuration(minutes)}
                      </span>
                    </div>
                  ))}
                </div>
                <div className="border-t border-zinc-200 pt-4">
                  <h2 className="text-sm font-semibold text-zinc-950">Preview</h2>
                  <div className="mt-3 min-h-16 rounded-md border border-zinc-200 bg-zinc-50 p-3 text-sm text-zinc-600">
                    {preview ? (
                      <div className="space-y-1">
                        <p className="font-medium text-zinc-950">
                          {preview.interaction}
                        </p>
                        <p>{preview.day}</p>
                        <p>
                          {formatMinutes(preview.startMinutes)}-
                          {formatMinutes(preview.endMinutes)}
                        </p>
                      </div>
                    ) : (
                      <p>Idle</p>
                    )}
                  </div>
                </div>
              </CardContent>
            </Card>
            <Card className="app-no-drag">
              <CardHeader>
                <CardTitle className="text-sm">Schedule</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-3 text-sm">
                  <div className="flex items-center justify-between gap-3">
                    <span className="text-muted-foreground">Period</span>
                    <span className="truncate font-medium">
                      {activePeriod
                        ? `${activePeriod.startDate} to ${activePeriod.endDate}`
                        : "No period"}
                    </span>
                  </div>
                  {activePeriod && (
                    <>
                      <div className="flex items-center justify-between gap-3">
                        <span className="text-muted-foreground">Cadence</span>
                        <span className="font-medium">
                          {formatCadence(activePeriod.cadence)}
                        </span>
                      </div>
                      <div className="flex items-center justify-between gap-3">
                        <span className="text-muted-foreground">Days</span>
                        <span className="font-medium">{visibleDayCount}</span>
                      </div>
                      <div className="flex items-center justify-between gap-3">
                        <span className="text-muted-foreground">Target</span>
                        <span className="font-medium">
                          {activePeriod.targetHoursPerDay}h/day
                        </span>
                      </div>
                    </>
                  )}
                  <div className="flex items-center justify-between gap-3">
                    <span className="text-muted-foreground">Events</span>
                    <span className="font-medium">
                      {eventsQuery.data?.length ?? 0}
                    </span>
                  </div>
                  <div className="flex items-center justify-between gap-3">
                    <span className="text-muted-foreground">Gap fills</span>
                    <span className="font-medium">
                      {gapFillsQuery.data?.length ?? 0}
                    </span>
                  </div>
                  <div className="flex items-center justify-between gap-3">
                    <span className="text-muted-foreground">Categories</span>
                    <span className="font-medium">{categories.length}</span>
                  </div>
                  <div className="flex items-center justify-between gap-3">
                    <span className="text-muted-foreground">Review</span>
                    <span className="font-medium">
                      {reviewItemsQuery.data?.length ?? 0}
                    </span>
                  </div>
                  {isBackendLoading && (
                    <p className="rounded-md border border-zinc-200 bg-white p-2 text-xs text-muted-foreground">
                      Loading backend data
                    </p>
                  )}
                  {backendError && (
                    <p className="rounded-md border border-destructive/30 bg-destructive/10 p-2 text-xs text-destructive">
                      {errorMessage(backendError)}
                    </p>
                  )}
                </div>
              </CardContent>
            </Card>
          </div>
        </section>
      </div>
    </main>
  );
}

export default App;
