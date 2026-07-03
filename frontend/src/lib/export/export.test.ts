import { describe, expect, it } from "vitest";
import type { Period } from "@/lib/api";
import type { ScheduleItem } from "@/lib/schedule/types";
import {
  defaultExportFilename,
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
    expect(summary.dailyTotals).toEqual([
      {
        date: "2026-06-01",
        categories: {
          Calendar: 120,
          Development: 120,
        },
      },
      {
        date: "2026-06-02",
        categories: {
          Development: 120,
        },
      },
    ]);
  });

  it("formats a copyable text summary", () => {
    const summary = buildPeriodExportSummary(items, period);
    const text = formatSummaryText(summary);

    expect(text).toContain("Period: Jun 1-Jun 2");
    expect(text).toContain("Calendar: 2h");
    expect(text).toContain("Development: 4h");
    expect(text).toContain("2026-06-01");
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

  it("builds a stable default export filename", () => {
    const summary = buildPeriodExportSummary(items, period);

    expect(defaultExportFilename(summary)).toBe(
      "clockr-2026-06-01-to-2026-06-02.csv",
    );
  });
});
