import type { CSSProperties } from "react";
import type { Period } from "@/lib/api";
import { categoryColorStyle } from "@/lib/category/colors";
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
      return "border-sky-300 bg-sky-50 text-sky-950 dark:border-sky-700 dark:bg-sky-950/40 dark:text-sky-100";
    case "gap":
      return "border-emerald-300 bg-emerald-50 text-emerald-950 dark:border-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-100";
    case "manual":
      return "border-amber-300 bg-amber-50 text-amber-950 dark:border-amber-700 dark:bg-amber-950/40 dark:text-amber-100";
    case "review":
      return "border-rose-300 bg-rose-50 text-rose-950 dark:border-rose-700 dark:bg-rose-950/40 dark:text-rose-100";
  }
}

export function scheduleItemPresentation(
  kind: ScheduleKind,
  categoryColor?: string,
): { className: string; style?: CSSProperties } {
  if (kind === "review") {
    return { className: kindClasses(kind) };
  }

  if (categoryColor) {
    return {
      className: "text-foreground",
      style: categoryColorStyle(categoryColor),
    };
  }

  return { className: kindClasses(kind) };
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
