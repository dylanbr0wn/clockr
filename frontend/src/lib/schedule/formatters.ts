import type { Period } from "@/lib/api";
import { formatMinutes } from "@/lib/scheduler";
import { formatDateKey } from "./date";
import type { ScheduleItem, ScheduleKind } from "./types";

export function formatPeriodLabel(period: Period) {
  return `${formatDateKey(period.startDate)}-${formatDateKey(period.endDate)}`;
}

export function formatCadence(value: string) {
  return value
    .split(/[-_\s]+/)
    .filter(Boolean)
    .map((part) => part.slice(0, 1).toUpperCase() + part.slice(1))
    .join(" ");
}

export function kindClasses(kind: ScheduleKind) {
  switch (kind) {
    case "calendar":
      return "border-sky-300 bg-sky-50 text-sky-950";
    case "gap":
      return "border-emerald-300 bg-emerald-50 text-emerald-950";
    case "manual":
      return "border-amber-300 bg-amber-50 text-amber-950";
    case "review":
      return "border-rose-300 bg-rose-50 text-rose-950";
    case "uncovered":
      return "border-dashed border-zinc-300 bg-background text-muted-foreground hover:border-zinc-400 hover:bg-muted/50";
  }
}

export function formatDuration(totalMinutes: number) {
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

export function durationLabel(item: ScheduleItem) {
  return formatDuration(item.endMinutes - item.startMinutes);
}

export function formatTimeRange(startMinutes: number, endMinutes: number) {
  return `${formatMinutes(startMinutes)}-${formatMinutes(endMinutes)}`;
}

export function errorMessage(error: unknown) {
  if (error instanceof Error) {
    return error.message;
  }

  return String(error);
}
