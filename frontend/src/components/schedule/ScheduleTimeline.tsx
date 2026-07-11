import { useMemo, useRef, useState } from "react";
import {
  Scheduler,
  type SchedulerCreateRequest,
} from "@/lib/scheduler";
import {
  SCHEDULE_END_MINUTES,
  SCHEDULE_START_MINUTES,
  WORKING_END_MINUTES,
  WORKING_START_MINUTES,
  type ScheduleDayMetadata,
  type ScheduleGapOverlay,
  type ScheduleMetadata,
} from "@/lib/schedule";
import { computeAllDayRowHeight } from "./ScheduleTimeline.helpers";
import type {
  ScheduleTimelineActions,
  ScheduleTimelineData,
} from "./ScheduleTimeline.types";
import { ScheduleTimelineGrid } from "./timeline/ScheduleTimelineGrid";
import { useInitialTimelineScroll } from "./useInitialTimelineScroll";
import { ScrollArea } from "../ui/scroll-area";


interface ScheduleTimelineProps {
  data: ScheduleTimelineData;
  actions: ScheduleTimelineActions;
}

function groupGapsByDay(visibleGaps: ScheduleGapOverlay[]) {
  const gapsByDay = new Map<string, ScheduleGapOverlay[]>();

  for (const gap of visibleGaps) {
    const gaps = gapsByDay.get(gap.day) ?? [];
    gaps.push(gap);
    gapsByDay.set(gap.day, gaps);
  }

  return gapsByDay;
}

export function ScheduleTimeline({ data, actions }: ScheduleTimelineProps) {
  const schedulerViewportRef = useRef<HTMLDivElement | null>(null);
  const backgroundRequestRef = useRef<SchedulerCreateRequest | null>(null);
  const [hoveredGapId, setHoveredGapId] = useState<string | null>(null);
  const timedItems = useMemo(
    () => data.items.filter((item) => !item.metadata?.isAllDay),
    [data.items],
  );
  const visibleGapsByDay = useMemo(
    () => groupGapsByDay(data.visibleGaps),
    [data.visibleGaps],
  );
  const allDayRowHeight = computeAllDayRowHeight(data.allDayChipsByDay);
  const hasAllDayRow = allDayRowHeight > 0;

  useInitialTimelineScroll(schedulerViewportRef);

  return (
    <Scheduler<ScheduleMetadata, ScheduleDayMetadata>
      days={data.days}
      items={timedItems}
      config={{
        maxDays: data.visibleDayCount,
        scheduleStartMinutes: SCHEDULE_START_MINUTES,
        scheduleEndMinutes: SCHEDULE_END_MINUTES,
        workingStartMinutes: WORKING_START_MINUTES,
        workingEndMinutes: WORKING_END_MINUTES,
      }}
      onCreate={actions.onCreate}
      onPreviewChange={actions.onPreviewChange}
      onCommitChange={actions.onCommitChange}
    >
      {(scheduler) => (
        <ScrollArea
          {...scheduler.getRootProps({
            ref: schedulerViewportRef,
            className:
              "app-no-drag h-full min-h-0 overflow-auto overscroll-none text-sm text-card-foreground bg-background-lighter",
          })}
          dir="ltr"
        >
          <ScheduleTimelineGrid
            scheduler={scheduler}
            allDayChipsByDay={data.allDayChipsByDay}
            visibleGapsByDay={visibleGapsByDay}
            resettableDays={data.resettableDays}
            visibleDayCount={data.visibleDayCount}
            aiConfigured={data.aiConfigured}
            allDayRowHeight={allDayRowHeight}
            hasAllDayRow={hasAllDayRow}
            hoveredGapId={hoveredGapId}
            backgroundRequestRef={backgroundRequestRef}
            onHoveredGapChange={setHoveredGapId}
            actions={actions}
          />
        </ScrollArea>
      )}
    </Scheduler>
  );
}
