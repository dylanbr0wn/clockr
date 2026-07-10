import { describe, expect, it } from "vitest";
import type { Period } from "@/lib/api";
import type { ScheduleItem } from "@/lib/schedule/types";
import {
  defaultExportFilename,
  formatDetailEntriesCSV,
  formatFlatDailyCSV,
  formatSummaryCSV,
  formatSummaryText,
} from "./formatters";
import { buildPeriodExportSummary } from "./summary";

const period: Period = {
  id: 1,
  startDate: "2026-06-01",
  endDate: "2026-06-02",
  cadence: "weekly",
  anchorDate: "2026-06-01",
  targetHoursPerDay: 8,
};

const items: ScheduleItem[] = [
  {
    id: "event-1",
    day: "2026-06-01",
    startMinutes: 9 * 60,
    endMinutes: 11 * 60,
    metadata: {
      title: "Meeting",
      category: "Calendar",
      categoryColor: "#0EA5E9",
      kind: "calendar",
    },
  },
  {
    id: "gap-fill-1",
    day: "2026-06-01",
    startMinutes: 13 * 60,
    endMinutes: 15 * 60,
    metadata: {
      title: "Deep work",
      category: "Development",
      categoryColor: "#8B5CF6",
      kind: "manual",
    },
  },
  {
    id: "gap-fill-2",
    day: "2026-06-02",
    startMinutes: 10 * 60,
    endMinutes: 12 * 60,
    metadata: {
      title: "Planning",
      category: "Development",
      categoryColor: "#8B5CF6",
      kind: "manual",
    },
  },
];

describe("period export", () => {
  it("builds period and daily totals from schedule items", () => {
    const summary = buildPeriodExportSummary(items, period);

    expect(summary.periodTotals).toEqual({
      Calendar: 120,
      Development: 240,
    });
    expect(summary.categoryColors).toEqual({
      Calendar: "#0EA5E9",
      Development: "#8B5CF6",
    });
    expect(summary.targetHoursPerDay).toBe(8);
    expect(summary.targetMinutes).toBe(8 * 60 * 2);
    expect(summary.actualMinutes).toBe(360);
    expect(summary.dailyTotals).toEqual([
      {
        date: "2026-06-01",
        categories: {
          Calendar: 120,
          Development: 120,
        },
        actualMinutes: 240,
        targetMinutes: 480,
      },
      {
        date: "2026-06-02",
        categories: {
          Development: 120,
        },
        actualMinutes: 120,
        targetMinutes: 480,
      },
    ]);
  });

  it("formats a copyable text summary", () => {
    const summary = buildPeriodExportSummary(items, period);
    const text = formatSummaryText(summary);

    expect(text).toContain("Period: Jun 1-Jun 2");
    expect(text).toContain("Target: 16h (8h/day)");
    expect(text).toContain("Actual: 6h");
    expect(text).toContain("Variance: -10h");
    expect(text).toContain("Calendar: 2h");
    expect(text).toContain("Development: 4h");
    expect(text).toContain("2026-06-01 — 4h / 8h target");
    expect(text).toContain("  Calendar: 2h");
  });

  it("formats a category-by-day CSV matrix", () => {
    const summary = buildPeriodExportSummary(items, period);
    const csv = formatSummaryCSV(summary);

    expect(csv).toBe(
      [
        "Category,2026-06-01,2026-06-02,Total",
        "Calendar,2.00,0.00,2.00",
        "Development,2.00,2.00,4.00",
      ].join("\n"),
    );
  });

  it("formats a flat daily category×day CSV", () => {
    const summary = buildPeriodExportSummary(items, period);
    const csv = formatFlatDailyCSV(summary, {
      Calendar: "CAL",
      Development: "DEV",
    });

    expect(csv).toBe(
      [
        "Date,Category,Key,Hours",
        "2026-06-01,Calendar,CAL,2.00",
        "2026-06-01,Development,DEV,2.00",
        "2026-06-02,Development,DEV,2.00",
      ].join("\n"),
    );
  });

  it("formats a detail-entries CSV", () => {
    const csv = formatDetailEntriesCSV(items, {
      Calendar: "CAL",
      Development: "DEV",
    });

    expect(csv).toBe(
      [
        "Start,End,Category,Key,Hours,Title",
        "2026-06-01T09:00,2026-06-01T11:00,Calendar,CAL,2.00,Meeting",
        "2026-06-01T13:00,2026-06-01T15:00,Development,DEV,2.00,Deep work",
        "2026-06-02T10:00,2026-06-02T12:00,Development,DEV,2.00,Planning",
      ].join("\n"),
    );
  });

  it("builds a stable default export filename", () => {
    const summary = buildPeriodExportSummary(items, period);

    expect(defaultExportFilename(summary)).toBe(
      "shiet-2026-06-01-to-2026-06-02.csv",
    );
  });
});
