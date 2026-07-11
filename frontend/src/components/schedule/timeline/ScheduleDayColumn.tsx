import { PlusIcon, RotateCcwIcon } from "lucide-react";
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from "@/components/ui/context-menu";
import { cn } from "@/lib/utils";
import { SchedulerItemLayer } from "@/lib/scheduler";
import type { ScheduleGapOverlay as ScheduleGapOverlayModel } from "@/lib/schedule";
import {
  NON_WORKING_HOURS_BACKGROUND,
  buildBackgroundCreateRequest,
  pointerClientYToSnappedMinute,
} from "../ScheduleTimeline.helpers";
import type { ScheduleTimelineActions } from "../ScheduleTimeline.types";
import { ScheduleGapOverlay } from "./ScheduleGapOverlay";
import { ScheduleTimedItem } from "./ScheduleTimedItem";
import {
  targetBlocksBackgroundMenu,
  targetBlocksTimelineHover,
  targetIgnoresTimelineCreate,
} from "./targetGuards";
import type {
  BackgroundRequestRef,
  TimelineDay,
  TimelineScheduler,
} from "./types";

interface ScheduleDayColumnProps {
  scheduler: TimelineScheduler;
  day: TimelineDay;
  gaps: ScheduleGapOverlayModel[];
  canResetDay: boolean;
  aiConfigured: boolean;
  timelineHeight: number;
  hourGridBackground: string;
  workingStartPercent: number;
  workingEndPercent: number;
  hoveredGapId: string | null;
  backgroundRequestRef: BackgroundRequestRef;
  onHoveredGapChange: (gapId: string | null) => void;
  actions: ScheduleTimelineActions;
}

export function ScheduleDayColumn({
  scheduler,
  day,
  gaps,
  canResetDay,
  aiConfigured,
  timelineHeight,
  hourGridBackground,
  workingStartPercent,
  workingEndPercent,
  hoveredGapId,
  backgroundRequestRef,
  onHoveredGapChange,
  actions,
}: ScheduleDayColumnProps) {
  const isWeekend = day.metadata?.isWeekend;

  return (
    <ContextMenu>
      <ContextMenuTrigger asChild>
        <div
          {...scheduler.getDayColumnProps(day, {
            onMouseMove: (event) => {
              if (!(event.target instanceof Element)) {
                onHoveredGapChange(null);
                return;
              }

              if (targetBlocksTimelineHover(event.target)) {
                onHoveredGapChange(null);
                return;
              }

              if (targetIgnoresTimelineCreate(event.target)) {
                return;
              }

              const minute = pointerClientYToSnappedMinute(
                event.clientY,
                event.currentTarget.getBoundingClientRect(),
                scheduler.visibleRange,
                scheduler.config.slotMinutes,
              );
              const hoveredGap = gaps.find(
                (gap) => minute >= gap.startMinutes && minute < gap.endMinutes,
              );

              onHoveredGapChange(hoveredGap?.id ?? null);
            },
            onMouseLeave: () => {
              onHoveredGapChange(null);
            },
            onContextMenu: (event) => {
              if (
                event.defaultPrevented ||
                !(event.target instanceof Element) ||
                targetBlocksBackgroundMenu(event.target)
              ) {
                return;
              }

              backgroundRequestRef.current = buildBackgroundCreateRequest({
                day: day.date,
                clientY: event.clientY,
                rect: event.currentTarget.getBoundingClientRect(),
                visibleRange: scheduler.visibleRange,
                slotMinutes: scheduler.config.slotMinutes,
                createDurationMinutes: scheduler.config.createDurationMinutes,
              });
            },
            className: cn([
              "relative not-last:border-r border-border",
              isWeekend ? "bg-background" : "bg-card",
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
                  background: NON_WORKING_HOURS_BACKGROUND,
                }}
              />
              <div
                className="pointer-events-none absolute inset-x-0 bottom-0 z-0"
                style={{
                  top: `${workingEndPercent}%`,
                  background: NON_WORKING_HOURS_BACKGROUND,
                }}
              />
              {[workingStartPercent, workingEndPercent].map((percent) => (
                <div
                  key={percent}
                  className="pointer-events-none absolute inset-x-0 z-1 border-t border-border"
                  style={{ top: `${percent}%` }}
                />
              ))}
            </>
          )}
          {gaps.map((gap) => (
            <ScheduleGapOverlay
              key={gap.id}
              gap={gap}
              isHovered={hoveredGapId === gap.id}
              aiConfigured={aiConfigured}
              scheduler={scheduler}
              onSelectGap={actions.onSelectGap}
            />
          ))}
          <SchedulerItemLayer scheduler={scheduler} day={day}>
            {(layoutItem) => (
              <ScheduleTimedItem
                key={layoutItem.item.id}
                scheduler={scheduler}
                layoutItem={layoutItem}
                actions={actions}
              />
            )}
          </SchedulerItemLayer>
        </div>
      </ContextMenuTrigger>
      <ContextMenuContent data-scheduler-ignore-create="">
        <ContextMenuItem
          onSelect={() => {
            const request = backgroundRequestRef.current;
            if (request?.day === day.date) {
              actions.onCreate(request);
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
          onSelect={() => actions.onResetDay(day.date)}
        >
          <RotateCcwIcon />
          Reset day
        </ContextMenuItem>
      </ContextMenuContent>
    </ContextMenu>
  );
}
