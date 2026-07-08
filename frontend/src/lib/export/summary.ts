import type { Period } from "@/lib/api";
import { buildDays, formatPeriodLabel } from "@/lib/schedule";
import { periodDayCount } from "@/lib/schedule/date";
import type { ScheduleItem } from "@/lib/schedule/types";

export interface DayCategoryHours {
  date: string;
  categories: Record<string, number>;
  actualMinutes: number;
  targetMinutes: number;
}

export interface PeriodExportSummary {
  periodLabel: string;
  startDate: string;
  endDate: string;
  targetHoursPerDay: number;
  targetMinutes: number;
  actualMinutes: number;
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
  const targetMinutesPerDay = period.targetHoursPerDay * 60;
  const targetMinutes = targetMinutesPerDay * dayCount;
  const actualMinutes = Object.values(periodTotals).reduce(
    (sum, minutes) => sum + minutes,
    0,
  );
  const dailyTotals = days.map(({ date }) => {
    const categories = dailyMap.get(date) ?? {};
    const dayActualMinutes = Object.values(categories).reduce(
      (sum, minutes) => sum + minutes,
      0,
    );

    return {
      date,
      categories,
      actualMinutes: dayActualMinutes,
      targetMinutes: targetMinutesPerDay,
    };
  });

  return {
    periodLabel: formatPeriodLabel(period),
    startDate: period.startDate,
    endDate: period.endDate,
    targetHoursPerDay: period.targetHoursPerDay,
    targetMinutes,
    actualMinutes,
    periodTotals,
    dailyTotals,
  };
}

export function periodProgressPercent(summary: PeriodExportSummary) {
  if (summary.targetMinutes <= 0) {
    return 0;
  }

  return Math.round((summary.actualMinutes / summary.targetMinutes) * 100);
}

export function varianceMinutes(actualMinutes: number, targetMinutes: number) {
  return actualMinutes - targetMinutes;
}

export function sortedCategories(summary: PeriodExportSummary) {
  return sortedCategoryNames(summary.periodTotals);
}

export function sortedCategoryNames(categories: Record<string, number>) {
  return Object.keys(categories).sort((left, right) => left.localeCompare(right));
}

export function sortCategoriesByMinutes(totals: Record<string, number>) {
  return Object.keys(totals).sort((left, right) => {
    const diff = totals[right] - totals[left];
    return diff !== 0 ? diff : left.localeCompare(right);
  });
}
