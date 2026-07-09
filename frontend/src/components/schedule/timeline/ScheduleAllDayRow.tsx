import { AlertTriangleIcon } from "lucide-react";
import { cn } from "@/lib/utils";
import {
  scheduleItemPresentation,
  type AllDayChip,
  type AllDaySpanPosition,
} from "@/lib/schedule";
import type { TimelineDay } from "./types";

interface ScheduleAllDayRowProps {
  days: TimelineDay[];
  allDayChipsByDay: Map<string, AllDayChip[]>;
  allDayRowHeight: number;
  visibleDayCount: number;
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

export function ScheduleAllDayRow({
  days,
  allDayChipsByDay,
  allDayRowHeight,
  visibleDayCount,
  onOpenReviewQueue,
}: ScheduleAllDayRowProps) {
  return (
    <>
      <div className="sticky left-0 top-[52px] z-30 flex items-center border-b border-r border-border bg-background px-2">
        <span className="text-[10px] font-medium uppercase tracking-wide text-muted-foreground">
          All day
        </span>
      </div>
      {days.map((day, index) => {
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
              const presentation = scheduleItemPresentation(
                chip.kind,
                chip.categoryColor,
              );
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
                    presentation.className,
                    isReview
                      ? "cursor-pointer hover:brightness-95"
                      : "cursor-default",
                  ])}
                  style={presentation.style}
                >
                  {isReview ? (
                    <div className="mb-0.5 flex items-center gap-1 text-[9px] font-semibold uppercase tracking-wide opacity-80">
                      <AlertTriangleIcon className="size-2.5" />
                      <span>Needs review</span>
                    </div>
                  ) : null}
                  <span className="truncate font-medium">{chip.title}</span>
                </button>
              );
            })}
          </div>
        );
      })}
    </>
  );
}
