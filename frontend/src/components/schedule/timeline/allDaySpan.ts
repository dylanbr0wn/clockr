import type { AllDayChip, AllDaySpanPosition } from "@/lib/schedule";

export function allDaySpanClasses(span: AllDaySpanPosition) {
  switch (span) {
    case "single":
      return "mx-1 rounded-md shadow-sm";
    case "start":
      return "ml-1 rounded-l-md rounded-r-none border-r-0";
    case "middle":
      return "rounded-none border-x-0";
    case "end":
      return "mr-1 rounded-r-md rounded-l-none border-l-0";
  }
}

export function resolveVisibleAllDaySpan(
  chip: AllDayChip,
  dayIndex: number,
  days: readonly { date: string }[],
  chipsByDay: Map<string, AllDayChip[]>,
): AllDaySpanPosition {
  const hasPrev =
    dayIndex > 0 &&
    (chipsByDay.get(days[dayIndex - 1]!.date) ?? []).some(
      (candidate) => candidate.eventId === chip.eventId,
    );
  const hasNext =
    dayIndex < days.length - 1 &&
    (chipsByDay.get(days[dayIndex + 1]!.date) ?? []).some(
      (candidate) => candidate.eventId === chip.eventId,
    );

  if (!hasPrev && !hasNext) {
    return "single";
  }

  if (!hasPrev) {
    return "start";
  }

  if (!hasNext) {
    return "end";
  }

  return "middle";
}
