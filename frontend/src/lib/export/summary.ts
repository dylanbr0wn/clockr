import type { Category, ExpectedTime, Period } from "@/lib/api";
import { buildDays, formatPeriodLabel } from "@/lib/schedule";
import { periodDayCount } from "@/lib/schedule/date";
import type { ScheduleItem } from "@/lib/schedule/types";

export interface BuildPeriodExportSummaryOptions {
  categories?: Category[];
  /** Per-date expected minutes from ExpectedTimeForRange. Required for targets. */
  expectedDays?: ExpectedTime[];
}

export interface DayCategoryHours {
  date: string;
  categories: Record<string, number>;
  actualMinutes: number;
  targetMinutes: number;
  /** ExpectedTime resolver source (`weekday` | `exception`). */
  source: string;
  /** Present when source is `exception`. */
  exceptionKind?: string;
}

export interface PeriodExportSummary {
  periodLabel: string;
  startDate: string;
  endDate: string;
  targetMinutes: number;
  actualMinutes: number;
  periodTotals: Record<string, number>;
  categoryColors: Record<string, string>;
  dailyTotals: DayCategoryHours[];
}

function buildCategoryColorMap(
  items: ScheduleItem[],
  categories: Category[] = [],
): Record<string, string> {
  const colors: Record<string, string> = {};

  for (const category of categories) {
    colors[category.name] = category.color;
  }

  for (const item of items) {
    const category = item.metadata?.category ?? "Unassigned";
    if (item.metadata?.categoryColor) {
      colors[category] = item.metadata.categoryColor;
    }
  }

  return colors;
}

function expectedTimeByDate(expectedDays: ExpectedTime[] = []) {
  const byDate = new Map<string, ExpectedTime>();
  for (const day of expectedDays) {
    byDate.set(day.date, day);
  }
  return byDate;
}

export function buildPeriodExportSummary(
  items: ScheduleItem[],
  period: Period,
  options: BuildPeriodExportSummaryOptions = {},
): PeriodExportSummary {
  const periodTotals: Record<string, number> = {};
  const dailyMap = new Map<string, Record<string, number>>();

  // Soft-demote: only confirmed TimeEntries count as payable.
  const payableItems = items.filter(
    (item) => item.metadata?.attestation === "confirmed",
  );

  for (const item of payableItems) {
    const category = item.metadata?.category ?? "Unassigned";
    const minutes = item.endMinutes - item.startMinutes;
    periodTotals[category] = (periodTotals[category] ?? 0) + minutes;

    const dayTotals = dailyMap.get(item.day) ?? {};
    dayTotals[category] = (dayTotals[category] ?? 0) + minutes;
    dailyMap.set(item.day, dayTotals);
  }

  const dayCount = periodDayCount(period);
  const days = buildDays(period.startDate, dayCount);
  const expectedByDate = expectedTimeByDate(options.expectedDays);
  const actualMinutes = Object.values(periodTotals).reduce(
    (sum, minutes) => sum + minutes,
    0,
  );
  let targetMinutes = 0;
  const dailyTotals = days.map(({ date }) => {
    const categories = dailyMap.get(date) ?? {};
    const dayActualMinutes = Object.values(categories).reduce(
      (sum, minutes) => sum + minutes,
      0,
    );
    const expected = expectedByDate.get(date);
    const dayTarget = expected?.expectedMinutes ?? 0;
    targetMinutes += dayTarget;

    return {
      date,
      categories,
      actualMinutes: dayActualMinutes,
      targetMinutes: dayTarget,
      source: expected?.source ?? "",
      ...(expected?.exceptionKind
        ? { exceptionKind: expected.exceptionKind }
        : {}),
    };
  });

  return {
    periodLabel: formatPeriodLabel(period),
    startDate: period.startDate,
    endDate: period.endDate,
    targetMinutes,
    actualMinutes,
    periodTotals,
    categoryColors: buildCategoryColorMap(payableItems, options.categories),
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
