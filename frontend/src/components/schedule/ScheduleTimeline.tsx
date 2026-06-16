import { useEffect, useRef } from "react";
import { cn } from "@/lib/utils";
import {
  CREATE_PREVIEW_ITEM_ID,
  Scheduler,
  SchedulerItemLayer,
  formatMinutes,
  type SchedulerCreateRequest,
} from "@/lib/scheduler";
import {
  INITIAL_SCROLL_CONTEXT_MINUTES,
  SCHEDULE_END_MINUTES,
  SCHEDULE_START_MINUTES,
  TIMELINE_HOUR_HEIGHT,
  WORKING_END_MINUTES,
  WORKING_START_MINUTES,
  durationLabel,
  formatTimeRange,
  kindClasses,
  type ScheduleChange,
  type ScheduleDay,
  type ScheduleDayMetadata,
  type ScheduleItem,
  type ScheduleMetadata,
} from "@/lib/schedule";

interface ScheduleTimelineProps {
  days: ScheduleDay[];
  items: ScheduleItem[];
  visibleDayCount: number;
  onCreate: (request: SchedulerCreateRequest) => void;
  onPreviewChange: (change: ScheduleChange | null) => void;
  onCommitChange: (change: ScheduleChange) => void;
  onEditItem: (item: ScheduleItem) => void;
}

export function ScheduleTimeline({
  days,
  items,
  visibleDayCount,
  onCreate,
  onPreviewChange,
  onCommitChange,
  onEditItem,
}: ScheduleTimelineProps) {
  const schedulerViewportRef = useRef<HTMLDivElement | null>(null);
  const didSetInitialScrollRef = useRef(false);

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
      items={items}
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
                    <p className="text-xs text-muted-foreground">
                      {day.date}
                    </p>
                  </div>
                </div>
              ))}

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
                          : "border-zinc-300 bg-zinc-50 text-zinc-950";

                        return (
                          <div
                            key={item.id}
                            {...scheduler.getItemProps(layoutItem, {
                              onDoubleClick: (event) => {
                                event.preventDefault();
                                event.stopPropagation();
                                onEditItem(item);
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
  );
}
