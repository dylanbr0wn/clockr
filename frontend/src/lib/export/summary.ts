import type { Period } from "@/lib/api";
import { buildDays, formatPeriodLabel } from "@/lib/schedule";
import { periodDayCount } from "@/lib/schedule/date";
import type { ScheduleItem } from "@/lib/schedule/types";

export interface DayCategoryHours {
  date: string;
  categories: Record<string, number>;
}

export interface PeriodExportSummary {
  periodLabel: string;
  startDate: string;
  endDate: string;
  periodTotals: Record<string, number>;
  dailyTotals: DayCategoryHours[];
}

export function buildPeriodExportSummary(
  items: ScheduleItem[],
  period: Period,
): PeriodExportSummary {
  const periodTotals: Record<string, number> = {};
  const dailyMap = new Map<string, Record<string, number>>();

  for (const item of items) {
    const category = item.metadata?.category ?? "Unassigned";
    const minutes = item.endMinutes - item.startMinutes;
    periodTotals[category] = (periodTotals[category] ?? 0) + minutes;

    const dayTotals = dailyMap.get(item.day) ?? {};
    dayTotals[category] = (dayTotals[category] ?? 0) + minutes;
    dailyMap.set(item.day, dayTotals);
  }

  const dayCount = periodDayCount(period);
  const days = buildDays(period.startDate, dayCount);
  const dailyTotals = days.map(({ date }) => ({
    date,
    categories: dailyMap.get(date) ?? {},
  }));

  return {
    periodLabel: formatPeriodLabel(period),
    startDate: period.startDate,
    endDate: period.endDate,
    periodTotals,
    dailyTotals,
  };
}

export function sortedCategories(summary: PeriodExportSummary) {
  return Object.keys(summary.periodTotals).sort((left, right) =>
    left.localeCompare(right),
  );
}
