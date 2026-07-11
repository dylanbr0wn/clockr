import { formatMinutes } from "@/lib/scheduler";
import {
  getTimeLabelClass,
  minuteToTimelinePercent,
} from "../ScheduleTimeline.helpers";
import type { TimelineScheduler } from "./types";

interface ScheduleTimeAxisProps {
  scheduler: TimelineScheduler;
  timelineMarks: number[];
}

export function ScheduleTimeAxis({
  scheduler,
  timelineMarks,
}: ScheduleTimeAxisProps) {
  return (
    <div className="sticky left-0 z-20 border-r border-border bg-background-lighter">
      {timelineMarks.map((minute) => (
        <div
          key={minute}
          data-scheduler-time={minute}
          className={getTimeLabelClass(minute, scheduler.visibleRange)}
          style={{
            top: `${minuteToTimelinePercent(minute, scheduler.visibleRange)}%`,
          }}
        >
          {formatMinutes(minute)}
        </div>
      ))}
    </div>
  );
}
