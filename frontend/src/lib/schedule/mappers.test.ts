import { describe, expect, it } from "vitest";
import type {
  Category,
  Event as ClockrEvent,
  GapFill,
  TzSegment,
} from "@/lib/api";
import {
  eventToSchedulerItem,
  gapFillToSchedulerItem,
  gapIntervalToSchedulerItem,
  periodContainsDate,
} from "./mappers";

const tzSegments: TzSegment[] = [
  {
    id: 1,
    periodId: 1,
    effectiveFromDate: "2026-06-01",
    ianaTz: "America/Vancouver",
  },
];

describe("schedule mappers", () => {
  it("maps all-day events across the whole schedule day", () => {
    const event: ClockrEvent = {
      id: 12,
      periodId: 1,
      calendarId: 3,
      provider: "google",
      externalId: "event-12",
      title: "Offsite",
      allDay: true,
      startDate: "2026-06-09",
      active: true,
    };

    expect(eventToSchedulerItem(event, tzSegments)).toMatchObject({
      id: "event-12",
      day: "2026-06-09",
      startMinutes: 0,
      endMinutes: 24 * 60,
      metadata: {
        title: "Offsite",
        category: "Calendar",
        kind: "calendar",
      },
    });
  });

  it("maps timed events into the active timezone position", () => {
    const event: ClockrEvent = {
      id: 34,
      periodId: 1,
      calendarId: 3,
      provider: "google",
      externalId: "event-34",
      title: "Focus",
      allDay: false,
      start: "2026-06-09T16:30:00Z",
      end: "2026-06-09T18:00:00Z",
      active: true,
    };

    expect(eventToSchedulerItem(event, tzSegments)).toMatchObject({
      id: "event-34",
      day: "2026-06-09",
      startMinutes: 9 * 60 + 30,
      endMinutes: 11 * 60,
    });
  });

  it("maps gap fills with category names and timezone-local minutes", () => {
    const categoriesById = new Map<number, Category>([
      [5, { id: 5, name: "Deep Work", isDefaultGap: true }],
    ]);
    const gapFill: GapFill = {
      id: 21,
      periodId: 1,
      day: "2026-06-09",
      start: "2026-06-09T18:00:00Z",
      end: "2026-06-09T19:15:00Z",
      categoryId: 5,
      note: "",
      source: "manual",
    };

    expect(
      gapFillToSchedulerItem(gapFill, categoriesById, tzSegments),
    ).toMatchObject({
      id: "gap-fill-21",
      day: "2026-06-09",
      startMinutes: 11 * 60,
      endMinutes: 12 * 60 + 15,
      metadata: {
        title: "Deep Work",
        category: "Deep Work",
        kind: "manual",
      },
    });
  });

  it("maps ai-confirmed gap fills with the gap kind", () => {
    const categoriesById = new Map<number, Category>([
      [5, { id: 5, name: "Deep Work", isDefaultGap: true }],
    ]);
    const gapFill: GapFill = {
      id: 22,
      periodId: 1,
      day: "2026-06-09",
      start: "2026-06-09T18:00:00Z",
      end: "2026-06-09T19:15:00Z",
      categoryId: 5,
      note: "Feature implementation",
      source: "gap",
    };

    expect(
      gapFillToSchedulerItem(gapFill, categoriesById, tzSegments),
    ).toMatchObject({
      metadata: {
        kind: "gap",
      },
    });
  });

  it("maps uncovered gap intervals for the schedule overlay", () => {
    expect(
      gapIntervalToSchedulerItem(
        "2026-06-09",
        {
          start: "2026-06-09T18:00:00Z",
          end: "2026-06-09T20:00:00Z",
        },
        tzSegments,
      ),
    ).toMatchObject({
      id: "gap-2026-06-09-2026-06-09T18:00:00.000Z-2026-06-09T20:00:00.000Z",
      day: "2026-06-09",
      startMinutes: 11 * 60,
      endMinutes: 13 * 60,
      disabled: true,
      metadata: {
        kind: "uncovered",
        gapWindowStart: "2026-06-09T18:00:00.000Z",
        gapWindowEnd: "2026-06-09T20:00:00.000Z",
      },
    });
  });

  it("detects whether a date belongs to a period", () => {
    const period = {
      id: 1,
      startDate: "2026-06-08",
      endDate: "2026-06-14",
      cadence: "weekly",
      anchorDate: "2026-06-08",
      targetHoursPerDay: 6,
    };

    expect(periodContainsDate(period, "2026-06-08")).toBe(true);
    expect(periodContainsDate(period, "2026-06-14")).toBe(true);
    expect(periodContainsDate(period, "2026-06-15")).toBe(false);
  });
});
