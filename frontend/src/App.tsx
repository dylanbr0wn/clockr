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
import { Clock } from "lucide-react";
import { Separator } from "./components/ui/separator";
import { Environment } from "../wailsjs/runtime/runtime";

type ScheduleKind = "calendar" | "gap" | "manual" | "review";

interface ScheduleMetadata {
  title: string;
  category: string;
  kind: ScheduleKind;
}

type DemoItem = SchedulerItem<ScheduleMetadata>;

const START_DATE = "2026-06-08";
const SCHEDULE_START_MINUTES = 0;
const SCHEDULE_END_MINUTES = 24 * 60;
const WORKING_START_MINUTES = 8 * 60;
const WORKING_END_MINUTES = 18 * 60;
const INITIAL_SCROLL_CONTEXT_MINUTES = 2 * 60;
const TIMELINE_HOUR_HEIGHT = 56;
const MAC_TITLEBAR_PADDING_CLASS = "pl-24";
const DEFAULT_TITLEBAR_PADDING_CLASS = "pl-3";

const initialItems: DemoItem[] = [
  {
    id: "planning",
    day: START_DATE,
    startMinutes: 8 * 60 + 30,
    endMinutes: 10 * 60,
    metadata: {
      title: "Sprint planning",
      category: "Product",
      kind: "calendar",
    },
  },
  {
    id: "review",
    day: START_DATE,
    startMinutes: 9 * 60 + 15,
    endMinutes: 10 * 60 + 30,
    metadata: {
      title: "Design review",
      category: "Client",
      kind: "review",
    },
  },
  {
    id: "deep-work",
    day: START_DATE,
    startMinutes: 13 * 60,
    endMinutes: 16 * 60,
    metadata: {
      title: "Implementation",
      category: "Engineering",
      kind: "gap",
    },
  },
  {
    id: "early-call",
    day: "2026-06-10",
    startMinutes: 7 * 60,
    endMinutes: 8 * 60,
    metadata: {
      title: "Vendor call",
      category: "Operations",
      kind: "calendar",
    },
  },
  {
    id: "late-writeup",
    day: "2026-06-12",
    startMinutes: 18 * 60,
    endMinutes: 19 * 60 + 15,
    metadata: {
      title: "Timesheet notes",
      category: "Admin",
      kind: "manual",
    },
  },
];

function buildDays(count: number): SchedulerDay[] {
  const [year, month, day] = START_DATE.split("-").map(Number);
  const start = new Date(Date.UTC(year, month - 1, day));

  return Array.from({ length: count }, (_, index) => {
    const date = new Date(start);
    date.setUTCDate(start.getUTCDate() + index);
    const isoDate = date.toISOString().slice(0, 10);
    return {
      date: isoDate,
      label: date.toLocaleDateString(undefined, {
        weekday: "short",
        month: "short",
        day: "numeric",
        timeZone: "UTC",
      }),
    };
  });
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

function durationLabel(item: DemoItem) {
  return formatDuration(item.endMinutes - item.startMinutes);
}

function getInitialPlatform() {
  return navigator.platform.toLowerCase().includes("mac") ? "darwin" : "";
}

function App() {
  const [dayCount, setDayCount] = useState(7);
  const [items, setItems] = useState<DemoItem[]>(initialItems);
  const [preview, setPreview] = useState<SchedulerChange<ScheduleMetadata> | null>(
    null,
  );
  const [platform, setPlatform] = useState(getInitialPlatform);
  const schedulerViewportRef = useRef<HTMLDivElement | null>(null);
  const didSetInitialScrollRef = useRef(false);
  const days = useMemo(() => buildDays(dayCount), [dayCount]);
  const titlebarPaddingClass =
    platform === "darwin"
      ? MAC_TITLEBAR_PADDING_CLASS
      : DEFAULT_TITLEBAR_PADDING_CLASS;
  const totals = useMemo(() => {
    return items.reduce<Record<string, number>>((next, item) => {
      const key = item.metadata?.category ?? "Unassigned";
      next[key] = (next[key] ?? 0) + item.endMinutes - item.startMinutes;
      return next;
    }, {});
  }, [items]);

  const handleCreate = (request: SchedulerCreateRequest) => {
    setItems((current) => [
      ...current,
      {
        id: `manual-${Date.now()}`,
        day: request.day,
        startMinutes: request.startMinutes,
        endMinutes: request.endMinutes,
        metadata: {
          title: "New block",
          category: "Manual",
          kind: "manual",
        },
      },
    ]);
  };

  const handleCommit = (change: SchedulerChange<ScheduleMetadata>) => {
    setItems((current) =>
      current.map((item) =>
        item.id === change.itemId
          ? {
              ...item,
              day: change.day,
              startMinutes: change.startMinutes,
              endMinutes: change.endMinutes,
            }
          : item,
      ),
    );
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
            <div className="flex rounded-md border border-zinc-200 bg-white p-1">
              {[1, 7, 14].map((count) => (
                <Button
                  key={count}
                  type="button"
                  variant={dayCount === count ? "default" : "ghost"}
                  size="sm"
                  onClick={() => setDayCount(count)}
                  aria-pressed={dayCount === count}
                >
                  {count}d
                </Button>
              ))}
            </div>
            <Button
              type="button"
              variant="outline"
              className="bg-white"
              onClick={() =>
                handleCreate({
                  day: days[0].date,
                  startMinutes: 11 * 60,
                  endMinutes: 12 * 60,
                })
              }
            >
              Block
            </Button>
          </div>
        </header>
        <Separator />
        <section className="grid min-h-0 flex-1 gap-4 lg:grid-cols-[minmax(0,1fr)_280px] p-3 bg-muted">
          <Scheduler
            days={days}
            items={items}
            config={{
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
              const dayBackground = [
                `repeating-linear-gradient(to bottom, transparent 0, transparent calc(${slotPercent}% - 1px), rgb(228 228 231) calc(${slotPercent}% - 1px), rgb(228 228 231) ${slotPercent}%)`,
                `linear-gradient(to bottom, rgb(250 250 250) 0%, rgb(250 250 250) ${workingStartPercent}%, transparent ${workingStartPercent}%, transparent ${workingEndPercent}%, rgb(250 250 250) ${workingEndPercent}%, rgb(250 250 250) 100%)`,
              ].join(", ");
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
                      "app-no-drag h-full min-h-0 overflow-auto overscroll-none rounded-md border border-border bg-background shadow-sm",
                  })}
                >
                  <div
                    className="grid"
                    style={{
                      minWidth: `${72 + scheduler.days.length * 138}px`,
                      gridTemplateColumns: `72px repeat(${scheduler.days.length}, minmax(138px, 1fr))`,
                      gridTemplateRows: `52px ${timelineHeight}px`,
                    }}
                  >
                    <div className="sticky left-0 top-0 z-40 border-b border-r border-border bg-background" />
                    {scheduler.days.map((day) => (
                      <div
                        key={day.date}
                        className="sticky top-0 z-30 flex items-center border-b border-r border-border bg-background px-3"
                      >
                        <div>
                          <p className="text-sm font-semibold text-zinc-950">
                            {day.label}
                          </p>
                          <p className="text-xs text-zinc-500">{day.date}</p>
                        </div>
                      </div>
                    ))}

                    <div
                      className="sticky left-0 z-20 border-r border-zinc-200 bg-background"
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

                    {scheduler.days.map((day) => (
                      <div
                        key={day.date}
                        {...scheduler.getDayColumnProps(day, {
                          className:
                            "relative border-r border-zinc-200 bg-white",
                          style: {
                            height: `${timelineHeight}px`,
                            backgroundImage: dayBackground,
                          },
                        })}
                      >
                        {[workingStartPercent, workingEndPercent].map((percent) => (
                          <div
                            key={percent}
                            className="pointer-events-none absolute inset-x-0 z-1 border-t border-border"
                            style={{ top: `${percent}%` }}
                          />
                        ))}
                        <SchedulerItemLayer scheduler={scheduler} day={day}>
                          {(layoutItem) => {
                            const item = layoutItem.item;

                            if (item.id === CREATE_PREVIEW_ITEM_ID) {
                              return (
                                <div
                                  key={item.id}
                                  {...scheduler.getItemProps(layoutItem, {
                                    className:
                                      "pointer-events-none z-20 flex flex-col justify-center rounded-md border-2 border-dashed border-zinc-400 bg-zinc-100/80 px-2 py-1 text-xs text-zinc-600",
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
                    ))}
                  </div>
                </div>
              );
            }}
          </Scheduler>

          <aside className="app-no-drag min-h-0 space-y-4 overflow-auto overscroll-none rounded-md border border-zinc-200 bg-white p-4 shadow-sm">
            <div>
              <h2 className="text-sm font-semibold text-zinc-950">Totals</h2>
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
          </aside>
        </section>
      </div>
    </main>
  );
}

export default App;
