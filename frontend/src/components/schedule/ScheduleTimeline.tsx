import { useEffect, useRef, useState } from "react";
import {
  AlertTriangleIcon,
  CopyIcon,
  PencilIcon,
  PlusIcon,
  RotateCcwIcon,
  SparklesIcon,
  Trash2Icon,
} from "lucide-react";
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from "@/components/ui/context-menu";
import { cn } from "@/lib/utils";
import {
  CREATE_PREVIEW_ITEM_ID,
  Scheduler,
  SchedulerItemLayer,
  MINUTES_PER_DAY,
  clamp,
  formatMinutes,
  snapMinutes,
  type SchedulerCreateRequest,
} from "@/lib/scheduler";
import {
  INITIAL_SCROLL_CONTEXT_MINUTES,
  ALL_DAY_CHIP_HEIGHT,
  ALL_DAY_ROW_MIN_HEIGHT,
  ALL_DAY_ROW_PADDING,
  SCHEDULE_END_MINUTES,
  SCHEDULE_START_MINUTES,
  TIMELINE_HOUR_HEIGHT,
  WORKING_END_MINUTES,
  WORKING_START_MINUTES,
  durationLabel,
  formatTimeRange,
  kindClasses,
  type AllDayChip,
  type AllDaySpanPosition,
  type ScheduleChange,
  type ScheduleDay,
  type ScheduleDayMetadata,
  type ScheduleGapOverlay,
  type ScheduleItem,
  type ScheduleMetadata,
} from "@/lib/schedule";

interface ScheduleTimelineProps {
  days: ScheduleDay[];
  items: ScheduleItem[];
  allDayChipsByDay: Map<string, AllDayChip[]>;
  visibleGaps: ScheduleGapOverlay[];
  resettableDays: ReadonlySet<string>;
  visibleDayCount: number;
  aiConfigured: boolean;
  onCreate: (request: SchedulerCreateRequest) => void;
  onPreviewChange: (change: ScheduleChange | null) => void;
  onCommitChange: (change: ScheduleChange) => void;
  onEditItem: (item: ScheduleItem) => void;
  onDuplicateItem: (item: ScheduleItem) => void;
  onRemoveItem: (item: ScheduleItem) => void;
  onResetDay: (day: string) => void;
  onSelectGap: (gap: ScheduleGapOverlay) => void;
  onOpenReviewQueue: () => void;
}

function allDaySpanClasses(span: AllDaySpanPosition) {
  switch (span) {
    case "single":
      return "rounded-md";
    case "start":
      return "rounded-l-md rounded-r-sm";
    case "middle":
      return "rounded-none";
    case "end":
      return "rounded-r-md rounded-l-sm";
  }
}

function computeAllDayRowHeight(allDayChipsByDay: Map<string, AllDayChip[]>) {
  let maxChipsPerDay = 0;

  for (const chips of allDayChipsByDay.values()) {
    maxChipsPerDay = Math.max(maxChipsPerDay, chips.length);
  }

  if (maxChipsPerDay === 0) {
    return 0;
  }

  const chipStackHeight =
    maxChipsPerDay * ALL_DAY_CHIP_HEIGHT + Math.max(0, maxChipsPerDay - 1) * 4;

  return Math.max(
    ALL_DAY_ROW_MIN_HEIGHT,
    ALL_DAY_ROW_PADDING * 2 + chipStackHeight,
  );
}

export function ScheduleTimeline({
  days,
  items,
  allDayChipsByDay,
  visibleGaps,
  resettableDays,
  visibleDayCount,
  aiConfigured,
  onCreate,
  onPreviewChange,
  onCommitChange,
  onEditItem,
  onDuplicateItem,
  onRemoveItem,
  onResetDay,
  onSelectGap,
  onOpenReviewQueue,
}: ScheduleTimelineProps) {
  const timedItems = items.filter((item) => !item.metadata?.isAllDay);
  const allDayRowHeight = computeAllDayRowHeight(allDayChipsByDay);
  const hasAllDayRow = allDayRowHeight > 0;
  const schedulerViewportRef = useRef<HTMLDivElement | null>(null);
  const didSetInitialScrollRef = useRef(false);
  const backgroundRequestRef = useRef<SchedulerCreateRequest | null>(null);
  const [hoveredGapId, setHoveredGapId] = useState<string | null>(null);

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

  return (
    <Scheduler<ScheduleMetadata, ScheduleDayMetadata>
      days={days}
      items={timedItems}
      config={{
        maxDays: visibleDayCount,
        scheduleStartMinutes: SCHEDULE_START_MINUTES,
        scheduleEndMinutes: SCHEDULE_END_MINUTES,
        workingStartMinutes: WORKING_START_MINUTES,
        workingEndMinutes: WORKING_END_MINUTES,
      }}
      onCreate={onCreate}
      onPreviewChange={onPreviewChange}
      onCommitChange={onCommitChange}
    >
      {(scheduler) => {
        const visibleDuration =
          scheduler.visibleRange.endMinutes -
          scheduler.visibleRange.startMinutes;
        const slotPercent = (60 / visibleDuration) * 100;
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
        const hourGridBackground = `repeating-linear-gradient(to bottom, transparent 0, transparent calc(${slotPercent}% - 1px), var(--border) calc(${slotPercent}% - 1px), var(--border) ${slotPercent}%)`;
        const nonWorkingHoursBackground =
          "repeating-linear-gradient(135deg, var(--background) 0 6px, transparent 6px 12px), var(--muted)";
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
            return "absolute right-3 translate-y-0 text-xs font-medium text-muted-foreground";
          }

          if (minute === scheduler.visibleRange.endMinutes) {
            return "absolute right-3 -translate-y-full text-xs font-medium text-muted-foreground";
          }

          return "absolute right-3 -translate-y-2 text-xs font-medium text-muted-foreground";
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
                gridTemplateRows: hasAllDayRow
                  ? `52px ${allDayRowHeight}px ${timelineHeight}px`
                  : `52px ${timelineHeight}px`,
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
                    <p className="text-xs text-muted-foreground">
                      {day.date}
                    </p>
                  </div>
                </div>
              ))}

              {hasAllDayRow ? (
                <>
                  <div className="sticky left-0 top-[52px] z-30 flex items-center border-b border-r border-border bg-background px-2">
                    <span className="text-[10px] font-medium uppercase tracking-wide text-muted-foreground">
                      All day
                    </span>
                  </div>
                  {scheduler.days.map((day, index) => {
                    const chips = allDayChipsByDay.get(day.date) ?? [];
                    const isWeekend = day.metadata?.isWeekend;

                    return (
                      <div
                        key={`all-day-${day.date}`}
                        className={cn([
                          "sticky top-[52px] z-20 flex flex-col gap-1 border-b border-border px-1 py-1",
                          isWeekend ? "bg-muted" : "bg-background",
                          index % visibleDayCount !== visibleDayCount - 1
                            ? "border-r"
                            : "",
                        ])}
                        style={{ height: `${allDayRowHeight}px` }}
                      >
                        {chips.map((chip) => {
                          const chipClass = kindClasses(chip.kind);
                          const isReview = chip.kind === "review";

                          return (
                            <button
                              key={chip.id}
                              type="button"
                              disabled={!isReview}
                              onClick={() => {
                                if (isReview) {
                                  onOpenReviewQueue();
                                }
                              }}
                              className={cn([
                                "flex min-h-6 w-full flex-col justify-center border px-2 py-0.5 text-left text-[11px] shadow-sm",
                                allDaySpanClasses(chip.allDaySpan),
                                chipClass,
                                isReview
                                  ? "cursor-pointer hover:brightness-95"
                                  : "cursor-default",
                              ])}
                            >
                              {isReview ? (
                                <div className="mb-0.5 flex items-center gap-1 text-[9px] font-semibold uppercase tracking-wide opacity-80">
                                  <AlertTriangleIcon className="size-2.5" />
                                  <span>Needs review</span>
                                </div>
                              ) : null}
                              <span className="truncate font-medium">
                                {chip.title}
                              </span>
                            </button>
                          );
                        })}
                      </div>
                    );
                  })}
                </>
              ) : null}

              <div className="sticky left-0 z-20 border-r border-border bg-background">
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
                const canResetDay = resettableDays.has(day.date);
                const dayGaps = visibleGaps.filter(
                  (gap) => gap.day === day.date,
                );

                return (
                  <ContextMenu key={day.date}>
                    <ContextMenuTrigger asChild>
                      <div
                        {...scheduler.getDayColumnProps(day, {
                          onMouseMove: (event) => {
                            const target = event.target;

                            if (!(target instanceof Element)) {
                              setHoveredGapId(null);
                              return;
                            }

                            if (target.closest("[data-scheduler-item]")) {
                              setHoveredGapId(null);
                              return;
                            }

                            if (
                              target.closest("[data-scheduler-ignore-create]")
                            ) {
                              return;
                            }

                            const rect =
                              event.currentTarget.getBoundingClientRect();
                            const percent =
                              rect.height <= 0
                                ? 0
                                : clamp(
                                    (event.clientY - rect.top) / rect.height,
                                    0,
                                    1,
                                  );
                            const rawMinutes =
                              scheduler.visibleRange.startMinutes +
                              percent *
                                (scheduler.visibleRange.endMinutes -
                                  scheduler.visibleRange.startMinutes);
                            const minute = snapMinutes(
                              rawMinutes,
                              scheduler.config.slotMinutes,
                            );
                            const hoveredGap = dayGaps.find(
                              (gap) =>
                                minute >= gap.startMinutes &&
                                minute < gap.endMinutes,
                            );

                            setHoveredGapId(hoveredGap?.id ?? null);
                          },
                          onMouseLeave: () => {
                            setHoveredGapId(null);
                          },
                          onContextMenu: (event) => {
                            const target = event.target;

                            if (
                              event.defaultPrevented ||
                              !(target instanceof Element) ||
                              target.closest(
                                "[data-scheduler-item], [data-scheduler-ignore-create]",
                              )
                            ) {
                              return;
                            }

                            const rect = event.currentTarget.getBoundingClientRect();
                            const percent =
                              rect.height <= 0
                                ? 0
                                : clamp((event.clientY - rect.top) / rect.height, 0, 1);
                            const rawMinutes =
                              scheduler.visibleRange.startMinutes +
                              percent *
                                (scheduler.visibleRange.endMinutes -
                                  scheduler.visibleRange.startMinutes);
                            const startMinutes = clamp(
                              snapMinutes(rawMinutes, scheduler.config.slotMinutes),
                              0,
                              MINUTES_PER_DAY - scheduler.config.createDurationMinutes,
                            );

                            backgroundRequestRef.current = {
                              day: day.date,
                              startMinutes,
                              endMinutes:
                                startMinutes + scheduler.config.createDurationMinutes,
                            };
                          },
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
                        {dayGaps.map((gap) => {
                          const top = minuteToPercent(gap.startMinutes);
                          const bottom = minuteToPercent(gap.endMinutes);
                          const height = Math.max(bottom - top, 1);
                          const isHovered = hoveredGapId === gap.id;

                          return (
                            <div
                              key={gap.id}
                              className={cn([
                                "pointer-events-none absolute inset-x-1 z-[5] rounded-md border border-dashed transition-[border-color,background-color,opacity] duration-150",
                                isHovered
                                  ? "border-zinc-300/70 bg-muted/10 opacity-100"
                                  : "border-transparent bg-transparent opacity-0",
                              ])}
                              style={{
                                top: `${top}%`,
                                height: `${height}%`,
                              }}
                            >
                              <button
                                type="button"
                                data-scheduler-ignore-create=""
                                className={cn([
                                  "absolute right-1 top-1 inline-flex size-6 items-center justify-center rounded-full border text-muted-foreground transition-[opacity,border-color,background-color,color,box-shadow] duration-150",
                                  isHovered
                                    ? "pointer-events-auto border-border/50 bg-background/60 opacity-35 shadow-none"
                                    : "pointer-events-none border-transparent bg-transparent opacity-0",
                                  "hover:opacity-100 hover:border-emerald-400/70 hover:bg-emerald-50/90 hover:text-emerald-900 hover:shadow-sm",
                                ])}
                                onClick={(event) => {
                                  event.preventDefault();
                                  event.stopPropagation();
                                  onSelectGap(gap);
                                }}
                                title={
                                  aiConfigured
                                    ? "Suggest gap fill"
                                    : "Configure AI for suggestions"
                                }
                              >
                                <SparklesIcon className="size-3" />
                              </button>
                            </div>
                          );
                        })}
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
                                  {formatTimeRange(
                                    item.startMinutes,
                                    item.endMinutes,
                                  )}
                                </div>
                              );
                            }

                            const metadata = item.metadata;
                            const itemClass = metadata
                              ? kindClasses(metadata.kind)
                              : "border-border bg-muted text-foreground";
                            const canMutateItem = item.id.startsWith("gap-fill-");

                            return (
                              <ContextMenu key={item.id}>
                                <ContextMenuTrigger asChild>
                                  <div
                                    {...scheduler.getItemProps(layoutItem, {
                                      onContextMenu: (event) => {
                                        event.stopPropagation();
                                      },
                                      onDoubleClick: (event) => {
                                        event.preventDefault();
                                        event.stopPropagation();
                                        onEditItem(item);
                                      },
                                      className: [
                                        "group z-10 flex min-h-10 cursor-grab flex-col overflow-hidden rounded-md border px-2 py-1 text-left text-xs shadow-sm transition-shadow active:cursor-grabbing",
                                        layoutItem.isPreview
                                          ? "opacity-70 ring-2 ring-background/20"
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
                                      {metadata?.kind === "review" ? (
                                        <div className="mb-1 flex items-center gap-1 text-[10px] font-semibold uppercase tracking-wide opacity-80">
                                          <AlertTriangleIcon className="size-3" />
                                          <span>Needs review</span>
                                        </div>
                                      ) : null}
                                      <p className="truncate font-semibold">
                                        {metadata?.title ?? "Untitled"}
                                      </p>
                                      <p className="truncate text-[11px] opacity-75">
                                        {formatTimeRange(
                                          item.startMinutes,
                                          item.endMinutes,
                                        )}{" "}
                                        · {durationLabel(item)}
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
                                </ContextMenuTrigger>
                                <ContextMenuContent data-scheduler-ignore-create="">
                                  <ContextMenuItem
                                    disabled={!canMutateItem}
                                    onSelect={() => onEditItem(item)}
                                  >
                                    <PencilIcon />
                                    Edit
                                  </ContextMenuItem>
                                  <ContextMenuItem
                                    disabled={!canMutateItem}
                                    onSelect={() => onDuplicateItem(item)}
                                  >
                                    <CopyIcon />
                                    Duplicate
                                  </ContextMenuItem>
                                  <ContextMenuSeparator />
                                  <ContextMenuItem
                                    disabled={!canMutateItem}
                                    variant="destructive"
                                    onSelect={() => onRemoveItem(item)}
                                  >
                                    <Trash2Icon />
                                    Remove
                                  </ContextMenuItem>
                                </ContextMenuContent>
                              </ContextMenu>
                            );
                          }}
                        </SchedulerItemLayer>
                      </div>
                    </ContextMenuTrigger>
                    <ContextMenuContent data-scheduler-ignore-create="">
                      <ContextMenuItem
                        onSelect={() => {
                          const request = backgroundRequestRef.current;
                          if (request?.day === day.date) {
                            onCreate(request);
                          }
                        }}
                      >
                        <PlusIcon />
                        New block
                      </ContextMenuItem>
                      <ContextMenuSeparator />
                      <ContextMenuItem
                        disabled={!canResetDay}
                        variant="destructive"
                        onSelect={() => onResetDay(day.date)}
                      >
                        <RotateCcwIcon />
                        Reset day
                      </ContextMenuItem>
                    </ContextMenuContent>
                  </ContextMenu>
                );
              })}
            </div>
          </div>
        );
      }}
    </Scheduler>
  );
}
