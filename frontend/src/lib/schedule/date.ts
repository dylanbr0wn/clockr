import type { Period } from "@/lib/api";
import type { ScheduleDay } from "./types";
import { FALLBACK_DAY_COUNT } from "./constants";

export function buildDays(startDate: string, count: number): ScheduleDay[] {
  const [year, month, day] = startDate.split("-").map(Number);
  const start = new Date(Date.UTC(year, month - 1, day));

  return Array.from({ length: count }, (_, index) => {
    const date = new Date(start);
    date.setUTCDate(start.getUTCDate() + index);
    const isoDate = date.toISOString().slice(0, 10);
    const dayOfWeek = date.getUTCDay();

    return {
      date: isoDate,
      label: date.toLocaleDateString(undefined, {
        weekday: "short",
        month: "short",
        day: "numeric",
        timeZone: "UTC",
      }),
      metadata: {
        isWeekend: dayOfWeek === 0 || dayOfWeek === 6,
      },
    };
  });
}

export function dateFromDateKey(value: string) {
  const [year, month, day] = value.split("-").map(Number);

  if (!year || !month || !day) {
    return null;
  }

  const date = new Date(Date.UTC(year, month - 1, day));
  return Number.isNaN(date.getTime()) ? null : date;
}

export function inclusiveDayCount(startDate: string, endDate: string) {
  const start = dateFromDateKey(startDate);
  const end = dateFromDateKey(endDate);

  if (!start || !end) {
    return 1;
  }

  const durationMs = end.getTime() - start.getTime();
  return Math.max(1, Math.floor(durationMs / 86_400_000) + 1);
}

export function periodDayCount(period: Period | null) {
  if (!period) {
    return FALLBACK_DAY_COUNT;
  }

  return inclusiveDayCount(period.startDate, period.endDate);
}

export function formatDateKey(value: string) {
  const date = dateFromDateKey(value);

  if (!date) {
    return value;
  }

  return date.toLocaleDateString(undefined, {
    month: "short",
    day: "numeric",
    timeZone: "UTC",
  });
}

export function localDateKey(date = new Date()) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

export function workingDaysRemaining(
  period: Pick<Period, "startDate" | "endDate">,
  today: string,
) {
  if (today > period.endDate) {
    return 0;
  }

  const dayCount = inclusiveDayCount(period.startDate, period.endDate);
  const days = buildDays(period.startDate, dayCount);

  return days.reduce((count, day) => {
    if (day.date < today || day.metadata?.isWeekend) {
      return count;
    }

    return count + 1;
  }, 0);
}
