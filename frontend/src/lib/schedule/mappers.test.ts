import { describe, expect, it } from "vitest";
import type {
  Category,
  Event as ShietEvent,
  TimeEntry,
  TzSegment,
} from "@/lib/api";
import {
  buildAllDayChipsByDay,
  buildEventCategoryOverlayMap,
  categoriesForAssignPicker,
  eventToSchedulerItem,
  expandAllDayEventDays,
  timeEntryToSchedulerItem,
  gapIntervalToOverlay,
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

const categoriesById = new Map<number, Category>([
  [5, {
    id: 5,
    name: "Deep Work",
    description: "",
    key: "Deep Work",
    color: "#8B5CF6",
    isDefaultGap: true,
    archived: false,
    inUse: false,
  }],
  [7, {
    id: 7,
    name: "Meetings",
    description: "",
    key: "Meetings",
    color: "#0EA5E9",
    isDefaultGap: false,
    archived: false,
    inUse: false,
  }],
]);

const emptyOverlays = new Map<string, number>();

describe("schedule mappers", () => {
  it("maps all-day events across the whole schedule day", () => {
    const event: ShietEvent = {
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

    expect(
      eventToSchedulerItem(
        event,
        tzSegments,
        categoriesById,
        undefined,
      ),
    ).toMatchObject({
      id: "event-12",
      day: "2026-06-09",
      startMinutes: 0,
      endMinutes: 24 * 60,
      disabled: true,
      metadata: {
        title: "Offsite",
        category: "Unassigned",
        kind: "calendar",
        isAllDay: true,
      },
    });
  });

  it("maps categorized calendar events with category color", () => {
    const event: ShietEvent = {
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
    const overlaysByKey = buildEventCategoryOverlayMap([
      {
        provider: "google",
        externalId: "event-34",
        categoryId: 7,
      },
    ]);

    expect(
      eventToSchedulerItem(
        event,
        tzSegments,
        categoriesById,
        overlaysByKey.get("google|event-34|"),
      ),
    ).toMatchObject({
      metadata: {
        category: "Meetings",
        categoryId: 7,
        categoryColor: "#0EA5E9",
        kind: "calendar",
      },
    });
  });

  it("expands all-day event days with exclusive end dates", () => {
    expect(
      expandAllDayEventDays({
        id: 1,
        periodId: 1,
        calendarId: 1,
        provider: "google",
        externalId: "single",
        title: "Single day",
        allDay: true,
        startDate: "2026-06-09",
        active: true,
      }),
    ).toEqual(["2026-06-09"]);

    expect(
      expandAllDayEventDays({
        id: 2,
        periodId: 1,
        calendarId: 1,
        provider: "google",
        externalId: "google-single",
        title: "Google single day",
        allDay: true,
        startDate: "2026-06-03",
        endDate: "2026-06-04",
        active: true,
      }),
    ).toEqual(["2026-06-03"]);

    expect(
      expandAllDayEventDays({
        id: 3,
        periodId: 1,
        calendarId: 1,
        provider: "google",
        externalId: "multi",
        title: "Multi day",
        allDay: true,
        startDate: "2026-06-03",
        endDate: "2026-06-06",
        active: true,
      }),
    ).toEqual(["2026-06-03", "2026-06-04", "2026-06-05"]);
  });

  it("builds all-day chips for visible days with span positions", () => {
    const events: ShietEvent[] = [
      {
        id: 12,
        periodId: 1,
        calendarId: 3,
        provider: "google",
        externalId: "event-12",
        title: "Offsite",
        allDay: true,
        startDate: "2026-06-09",
        endDate: "2026-06-12",
        active: true,
      },
      {
        id: 34,
        periodId: 1,
        calendarId: 3,
        provider: "google",
        externalId: "event-34",
        title: "Holiday",
        allDay: true,
        startDate: "2026-06-15",
        active: true,
      },
    ];
    const visibleDays = new Set(["2026-06-09", "2026-06-10", "2026-06-11"]);
    const chipsByDay = buildAllDayChipsByDay(
      events,
      visibleDays,
      categoriesById,
      emptyOverlays,
      new Map(),
    );

    expect(chipsByDay.get("2026-06-09")).toMatchObject([
      {
        id: "event-12@2026-06-09",
        title: "Offsite",
        allDaySpan: "start",
      },
    ]);
    expect(chipsByDay.get("2026-06-10")).toMatchObject([
      {
        id: "event-12@2026-06-10",
        allDaySpan: "middle",
      },
    ]);
    expect(chipsByDay.get("2026-06-11")).toMatchObject([
      {
        id: "event-12@2026-06-11",
        allDaySpan: "end",
      },
    ]);
    expect(chipsByDay.has("2026-06-15")).toBe(false);
  });

  it("maps timed events into the active timezone position", () => {
    const event: ShietEvent = {
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

    expect(
      eventToSchedulerItem(event, tzSegments, categoriesById, undefined),
    ).toMatchObject({
      id: "event-34",
      day: "2026-06-09",
      startMinutes: 9 * 60 + 30,
      endMinutes: 11 * 60,
      disabled: true,
    });
  });

  it("marks open review events distinctly on the schedule", () => {
    const event: ShietEvent = {
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

    expect(
      eventToSchedulerItem(
        event,
        tzSegments,
        categoriesById,
        7,
        undefined,
        {
          reviewItemId: 12,
          kind: "new_in_gap",
        },
      ),
    ).toMatchObject({
      id: "event-34",
      disabled: true,
      metadata: {
        category: "Needs review",
        kind: "review",
        reviewItemId: 12,
        reviewKind: "new_in_gap",
        mutable: false,
        excludable: false,
        opensReviewQueue: true,
      },
    });
  });

  it("maps time entries with category names and timezone-local minutes", () => {
    const timeEntry: TimeEntry = {
      id: 21,
      periodId: 1,
      localWorkDate: "2026-06-09",
      start: "2026-06-09T18:00:00Z",
      end: "2026-06-09T19:15:00Z",
      durationMinutes: 75,
      categoryId: 5,
      description: "",
      attestation: "confirmed",
    };

    expect(
      timeEntryToSchedulerItem(timeEntry, categoriesById, tzSegments),
    ).toMatchObject({
      id: "time-entry-21",
      day: "2026-06-09",
      startMinutes: 11 * 60,
      endMinutes: 12 * 60 + 15,
      metadata: {
        title: "Deep Work",
        category: "Deep Work",
        categoryColor: "#8B5CF6",
        kind: "manual",
        mutable: true,
        excludable: false,
        opensReviewQueue: false,
      },
    });
  });

  it("maps gap_fill time entries with the gap kind", () => {
    const timeEntry: TimeEntry = {
      id: 22,
      periodId: 1,
      localWorkDate: "2026-06-09",
      start: "2026-06-09T18:00:00Z",
      end: "2026-06-09T19:15:00Z",
      durationMinutes: 75,
      categoryId: 5,
      description: "Feature implementation",
      attestation: "confirmed",
      method: "gap_fill",
    };

    expect(
      timeEntryToSchedulerItem(timeEntry, categoriesById, tzSegments),
    ).toMatchObject({
      metadata: {
        kind: "gap",
        categoryColor: "#8B5CF6",
      },
    });
  });

  it("maps uncovered gap intervals for the schedule overlay", () => {
    expect(
      gapIntervalToOverlay(
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
      gapWindowStart: "2026-06-09T18:00:00.000Z",
      gapWindowEnd: "2026-06-09T20:00:00.000Z",
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

  it("excludes archived categories from assign pickers unless selected", () => {
    const categories: Category[] = [
      {
        id: 1,
        name: "Active",
        description: "",
        key: "Active",
        color: "#0EA5E9",
        isDefaultGap: false,
        archived: false,
        inUse: false,
      },
      {
        id: 2,
        name: "Archived",
        description: "",
        key: "Archived",
        color: "#64748B",
        isDefaultGap: false,
        archived: true,
        inUse: true,
      },
      {
        id: 3,
        name: "Other archived",
        description: "",
        key: "Other",
        color: "#94A3B8",
        isDefaultGap: false,
        archived: true,
        inUse: true,
      },
    ];

    expect(categoriesForAssignPicker(categories).map((c) => c.id)).toEqual([1]);
    expect(categoriesForAssignPicker(categories, 2).map((c) => c.id)).toEqual([
      1, 2,
    ]);
  });
});
