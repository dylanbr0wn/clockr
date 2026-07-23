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
};

const expectedDays = [
  {
    date: "2026-06-01",
    expectedMinutes: 480,
    windows: [{ startMinutes: 540, endMinutes: 1020 }],
    source: "weekday",
  },
  {
    date: "2026-06-02",
    expectedMinutes: 480,
    windows: [{ startMinutes: 540, endMinutes: 1020 }],
    source: "weekday",
  },
];

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
    id: "time-entry-1",
    day: "2026-06-01",
    startMinutes: 13 * 60,
    endMinutes: 15 * 60,
    metadata: {
      title: "Deep work",
      category: "Development",
      categoryColor: "#8B5CF6",
      kind: "manual",
      attestation: "confirmed",
    },
  },
  {
    id: "time-entry-2",
    day: "2026-06-02",
    startMinutes: 10 * 60,
    endMinutes: 12 * 60,
    metadata: {
      title: "Planning",
      category: "Development",
      categoryColor: "#8B5CF6",
      kind: "manual",
      attestation: "confirmed",
    },
  },
];

describe("period export", () => {
  it("builds period and daily totals from confirmed time entries only", () => {
    const summary = buildPeriodExportSummary(items, period, { expectedDays });

    expect(summary.periodTotals).toEqual({
      Development: 240,
    });
    expect(summary.categoryColors).toEqual({
      Development: "#8B5CF6",
    });
    expect(summary.targetMinutes).toBe(8 * 60 * 2);
    expect(summary.actualMinutes).toBe(240);
    expect(summary.dailyTotals).toEqual([
      {
        date: "2026-06-01",
        categories: {
          Development: 120,
        },
        actualMinutes: 120,
        targetMinutes: 480,
        source: "weekday",
      },
      {
        date: "2026-06-02",
        categories: {
          Development: 120,
        },
        actualMinutes: 120,
        targetMinutes: 480,
        source: "weekday",
      },
    ]);
  });

  it("excludes draft and dismissed time entries from payable totals", () => {
    const mixed: ScheduleItem[] = [
      {
        id: "draft-1",
        day: "2026-06-01",
        startMinutes: 9 * 60,
        endMinutes: 10 * 60,
        metadata: {
          title: "Draft",
          category: "Development",
          kind: "manual",
          attestation: "draft",
        },
      },
      {
        id: "dismissed-1",
        day: "2026-06-01",
        startMinutes: 10 * 60,
        endMinutes: 11 * 60,
        metadata: {
          title: "Dismissed",
          category: "Development",
          kind: "manual",
          attestation: "dismissed",
        },
      },
      {
        id: "confirmed-1",
        day: "2026-06-01",
        startMinutes: 11 * 60,
        endMinutes: 12 * 60,
        metadata: {
          title: "Confirmed",
          category: "Development",
          kind: "manual",
          attestation: "confirmed",
        },
      },
    ];

    const summary = buildPeriodExportSummary(mixed, period, { expectedDays });
    expect(summary.actualMinutes).toBe(60);
    expect(summary.periodTotals).toEqual({ Development: 60 });
  });

  it("carries exception source and kind onto daily totals", () => {
    const summary = buildPeriodExportSummary([], period, {
      expectedDays: [
        {
          date: "2026-06-01",
          expectedMinutes: 0,
          windows: [],
          source: "exception",
          exceptionKind: "holiday",
        },
        {
          date: "2026-06-02",
          expectedMinutes: 240,
          windows: [],
          source: "exception",
          exceptionKind: "changed_hours",
        },
      ],
    });

    expect(summary.dailyTotals).toEqual([
      {
        date: "2026-06-01",
        categories: {},
        actualMinutes: 0,
        targetMinutes: 0,
        source: "exception",
        exceptionKind: "holiday",
      },
      {
        date: "2026-06-02",
        categories: {},
        actualMinutes: 0,
        targetMinutes: 240,
        source: "exception",
        exceptionKind: "changed_hours",
      },
    ]);
  });

  it("sums only ExpectedTime targets (weekends stay zero)", () => {
    const weekendPeriod: Period = {
      id: 2,
      startDate: "2026-06-05",
      endDate: "2026-06-07",
      cadence: "weekly",
      anchorDate: "2026-06-05",
    };
    const summary = buildPeriodExportSummary([], weekendPeriod, {
      expectedDays: [
        {
          date: "2026-06-05",
          expectedMinutes: 480,
          windows: [],
          source: "weekday",
        },
        {
          date: "2026-06-06",
          expectedMinutes: 0,
          windows: [],
          source: "weekday",
        },
        {
          date: "2026-06-07",
          expectedMinutes: 0,
          windows: [],
          source: "weekday",
        },
      ],
    });

    expect(summary.targetMinutes).toBe(480);
    expect(summary.dailyTotals.map((day) => day.targetMinutes)).toEqual([
      480, 0, 0,
    ]);
  });

  it("formats a copyable text summary", () => {
    const summary = buildPeriodExportSummary(items, period, { expectedDays });
    const text = formatSummaryText(summary);

    expect(text).toContain("Period: Jun 1-Jun 2");
    expect(text).toContain("Target: 16h");
    expect(text).not.toContain("h/day");
    expect(text).toContain("Actual: 4h");
    expect(text).toContain("Variance: -12h");
    expect(text).not.toContain("Calendar:");
    expect(text).toContain("Development: 4h");
    expect(text).toContain("2026-06-01 — 2h / 8h target");
    expect(text).toContain("  Development: 2h");
  });

  it("formats a category-by-day CSV matrix", () => {
    const summary = buildPeriodExportSummary(items, period, { expectedDays });
    const csv = formatSummaryCSV(summary);

    expect(csv).toBe(
      [
        "Category,2026-06-01,2026-06-02,Total",
        "Development,2.00,2.00,4.00",
      ].join("\n"),
    );
  });

  it("formats a flat daily category×day CSV", () => {
    const summary = buildPeriodExportSummary(items, period, { expectedDays });
    const csv = formatFlatDailyCSV(summary, {
      Calendar: "CAL",
      Development: "DEV",
    });

    expect(csv).toBe(
      [
        "Date,Category,Key,Hours",
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
        "2026-06-01T13:00,2026-06-01T15:00,Development,DEV,2.00,Deep work",
        "2026-06-02T10:00,2026-06-02T12:00,Development,DEV,2.00,Planning",
      ].join("\n"),
    );
  });

  it("builds a stable default export filename", () => {
    const summary = buildPeriodExportSummary(items, period, { expectedDays });

    expect(defaultExportFilename(summary)).toBe(
      "shiet-2026-06-01-to-2026-06-02.csv",
    );
  });
});
