import { formatDuration } from "@/lib/schedule";
import type { ScheduleItem } from "@/lib/schedule/types";
import {
  sortedCategories,
  varianceMinutes,
  type PeriodExportSummary,
} from "./summary";

export function formatDecimalHours(minutes: number) {
  const hours = minutes / 60;
  const rounded = Math.round(hours * 10) / 10;

  if (Number.isInteger(rounded)) {
    return `${rounded}h`;
  }

  return `${rounded.toFixed(1)}h`;
}

function minutesToDecimalHours(minutes: number) {
  return (minutes / 60).toFixed(2);
}

function escapeCsvCell(value: string) {
  if (/[",\n]/.test(value)) {
    return `"${value.replace(/"/g, '""')}"`;
  }

  return value;
}

export function formatSignedMinutes(minutes: number) {
  const sign = minutes >= 0 ? "+" : "-";
  return `${sign}${formatDuration(Math.abs(minutes))}`;
}

export function formatVariance(actualMinutes: number, targetMinutes: number) {
  return formatSignedMinutes(varianceMinutes(actualMinutes, targetMinutes));
}

export function formatSummaryText(summary: PeriodExportSummary) {
  const variance = varianceMinutes(summary.actualMinutes, summary.targetMinutes);
  const lines: string[] = [
    `Period: ${summary.periodLabel}`,
    `${summary.startDate} to ${summary.endDate}`,
    "",
    `Target: ${formatDuration(summary.targetMinutes)} (${summary.targetHoursPerDay}h/day)`,
    `Actual: ${formatDuration(summary.actualMinutes)}`,
    `Variance: ${formatSignedMinutes(variance)}`,
    "",
    "Totals by category:",
  ];

  for (const category of sortedCategories(summary)) {
    lines.push(
      `  ${category}: ${formatDuration(summary.periodTotals[category])}`,
    );
  }

  lines.push("", "Daily breakdown:");

  for (const day of summary.dailyTotals) {
    const categories = Object.entries(day.categories).sort(([left], [right]) =>
      left.localeCompare(right),
    );

    lines.push(
      `${day.date} — ${formatDuration(day.actualMinutes)} / ${formatDuration(day.targetMinutes)} target`,
    );

    if (categories.length === 0) {
      lines.push("  (no tracked time)");
      lines.push("");
      continue;
    }

    for (const [category, minutes] of categories) {
      lines.push(`  ${category}: ${formatDuration(minutes)}`);
    }
    lines.push("");
  }

  return lines.join("\n").trimEnd();
}

export function formatSummaryCSV(summary: PeriodExportSummary) {
  const categories = sortedCategories(summary);
  const header = [
    "Category",
    ...summary.dailyTotals.map((day) => day.date),
    "Total",
  ];

  const rows = categories.map((category) => {
    let totalMinutes = 0;
    const dayValues = summary.dailyTotals.map((day) => {
      const minutes = day.categories[category] ?? 0;
      totalMinutes += minutes;
      return minutesToDecimalHours(minutes);
    });

    return [
      category,
      ...dayValues,
      minutesToDecimalHours(totalMinutes),
    ];
  });

  return [header, ...rows]
    .map((row) => row.map((cell) => escapeCsvCell(cell)).join(","))
    .join("\n");
}

/** Flat daily rollup: one row per category×day. */
export function formatFlatDailyCSV(
  summary: PeriodExportSummary,
  categoryKeys: Record<string, string> = {},
) {
  const header = ["Date", "Category", "Key", "Hours"];
  const rows: string[][] = [];

  for (const day of summary.dailyTotals) {
    const categories = Object.entries(day.categories).sort(([left], [right]) =>
      left.localeCompare(right),
    );
    for (const [category, minutes] of categories) {
      rows.push([
        day.date,
        category,
        categoryKeys[category] ?? category,
        minutesToDecimalHours(minutes),
      ]);
    }
  }

  return [header, ...rows]
    .map((row) => row.map((cell) => escapeCsvCell(cell)).join(","))
    .join("\n");
}

function entryDateTime(day: string, minutes: number) {
  const hours = Math.floor(minutes / 60);
  const mins = minutes % 60;
  return `${day}T${String(hours).padStart(2, "0")}:${String(mins).padStart(2, "0")}`;
}

/** Detail grain: one row per schedule entry (event or gap fill). */
export function formatDetailEntriesCSV(
  items: ScheduleItem[],
  categoryKeys: Record<string, string> = {},
) {
  const header = ["Start", "End", "Category", "Key", "Hours", "Title"];
  const sorted = [...items].sort((left, right) => {
    if (left.day !== right.day) {
      return left.day.localeCompare(right.day);
    }
    if (left.startMinutes !== right.startMinutes) {
      return left.startMinutes - right.startMinutes;
    }
    return left.id.localeCompare(right.id);
  });

  const rows = sorted.map((item) => {
    const category = item.metadata?.category ?? "Unassigned";
    const title = item.metadata?.title ?? "";
    const minutes = item.endMinutes - item.startMinutes;
    return [
      entryDateTime(item.day, item.startMinutes),
      entryDateTime(item.day, item.endMinutes),
      category,
      categoryKeys[category] ?? category,
      minutesToDecimalHours(minutes),
      title,
    ];
  });

  return [header, ...rows]
    .map((row) => row.map((cell) => escapeCsvCell(cell)).join(","))
    .join("\n");
}

export function defaultExportFilename(summary: PeriodExportSummary) {
  return `shiet-${summary.startDate}-to-${summary.endDate}.csv`;
}

export const EXPORT_TEMPLATE_MATRIX_CSV = "matrix_csv";
export const EXPORT_TEMPLATE_FLAT_DAILY_CSV = "flat_daily_csv";
export const EXPORT_TEMPLATE_DETAIL_ENTRIES_CSV = "detail_entries_csv";
export const EXPORT_TEMPLATE_TEXT_SUMMARY = "text_summary";
