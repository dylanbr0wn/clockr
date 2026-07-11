import { cn } from "@/lib/utils";
import type { TimelineDay } from "./types";

interface ScheduleDayHeadersProps {
  days: TimelineDay[];
  visibleDayCount: number;
}

export function ScheduleDayHeaders({
  days,
  visibleDayCount,
}: ScheduleDayHeadersProps) {
  return (
    <>
      {days.map((day, index) => (
        <div
          key={day.date}
          className={cn([
            "sticky top-0 z-40 flex items-center border-b border-border px-3",
            day.metadata?.isWeekend ? "bg-background" : "bg-background-lighter",
            index % visibleDayCount !== visibleDayCount - 1 ? "border-r" : "",
          ])}
        >
          <div>
            <p className="text-sm font-semibold text-foreground">{day.label}</p>
            <p className="text-xs text-muted-foreground">{day.date}</p>
          </div>
        </div>
      ))}
    </>
  );
}
