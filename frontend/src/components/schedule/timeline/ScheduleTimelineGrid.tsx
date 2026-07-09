import type { AllDayChip, ScheduleGapOverlay } from "@/lib/schedule";
import {
  DAY_COLUMN_MIN_WIDTH,
  DAY_HEADER_ROW_HEIGHT,
  TIME_AXIS_COLUMN_WIDTH,
  buildHourGridBackground,
  buildTimelineMarks,
  computeTimelineHeight,
  minuteToTimelinePercent,
} from "../ScheduleTimeline.helpers";
import type { ScheduleTimelineActions } from "../ScheduleTimeline.types";
import { ScheduleAllDayRow } from "./ScheduleAllDayRow";
import { ScheduleDayColumn } from "./ScheduleDayColumn";
import { ScheduleDayHeaders } from "./ScheduleDayHeaders";
import { ScheduleTimeAxis } from "./ScheduleTimeAxis";
import type { BackgroundRequestRef, TimelineScheduler } from "./types";

interface ScheduleTimelineGridProps {
  scheduler: TimelineScheduler;
  allDayChipsByDay: Map<string, AllDayChip[]>;
  visibleGapsByDay: Map<string, ScheduleGapOverlay[]>;
  resettableDays: ReadonlySet<string>;
  visibleDayCount: number;
  aiConfigured: boolean;
  allDayRowHeight: number;
  hasAllDayRow: boolean;
  hoveredGapId: string | null;
  backgroundRequestRef: BackgroundRequestRef;
  onHoveredGapChange: (gapId: string | null) => void;
  actions: ScheduleTimelineActions;
}

export function ScheduleTimelineGrid({
  scheduler,
  allDayChipsByDay,
  visibleGapsByDay,
  resettableDays,
  visibleDayCount,
  aiConfigured,
  allDayRowHeight,
  hasAllDayRow,
  hoveredGapId,
  backgroundRequestRef,
  onHoveredGapChange,
  actions,
}: ScheduleTimelineGridProps) {
  const timelineHeight = computeTimelineHeight(scheduler.visibleRange);
  const hourGridBackground = buildHourGridBackground(scheduler.visibleRange);
  const workingStartPercent = minuteToTimelinePercent(
    scheduler.config.workingStartMinutes,
    scheduler.visibleRange,
  );
  const workingEndPercent = minuteToTimelinePercent(
    scheduler.config.workingEndMinutes,
    scheduler.visibleRange,
  );
  const timelineMarks = buildTimelineMarks(scheduler.visibleRange);

  return (
    <div
      className="grid"
      style={{
        minWidth: `${
          TIME_AXIS_COLUMN_WIDTH + scheduler.days.length * DAY_COLUMN_MIN_WIDTH
        }px`,
        gridTemplateColumns: `${TIME_AXIS_COLUMN_WIDTH}px repeat(${scheduler.days.length}, minmax(${DAY_COLUMN_MIN_WIDTH}px, 1fr))`,
        gridTemplateRows: hasAllDayRow
          ? `${DAY_HEADER_ROW_HEIGHT}px ${allDayRowHeight}px ${timelineHeight}px`
          : `${DAY_HEADER_ROW_HEIGHT}px ${timelineHeight}px`,
      }}
    >
      <div className="sticky left-0 top-0 z-40 border-b border-r border-border bg-background" />
      <ScheduleDayHeaders
        days={scheduler.days}
        visibleDayCount={visibleDayCount}
      />
      {hasAllDayRow ? (
        <ScheduleAllDayRow
          days={scheduler.days}
          allDayChipsByDay={allDayChipsByDay}
          allDayRowHeight={allDayRowHeight}
          onOpenReviewQueue={actions.onOpenReviewQueue}
        />
      ) : null}
      <ScheduleTimeAxis scheduler={scheduler} timelineMarks={timelineMarks} />
      {scheduler.days.map((day) => (
        <ScheduleDayColumn
          key={day.date}
          scheduler={scheduler}
          day={day}
          gaps={visibleGapsByDay.get(day.date) ?? []}
          canResetDay={resettableDays.has(day.date)}
          aiConfigured={aiConfigured}
          timelineHeight={timelineHeight}
          hourGridBackground={hourGridBackground}
          workingStartPercent={workingStartPercent}
          workingEndPercent={workingEndPercent}
          hoveredGapId={hoveredGapId}
          backgroundRequestRef={backgroundRequestRef}
          onHoveredGapChange={onHoveredGapChange}
          actions={actions}
        />
      ))}
    </div>
  );
}
