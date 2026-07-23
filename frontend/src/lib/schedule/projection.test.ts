import { describe, expect, it } from "vitest";
import {
  buildResettableDays,
  buildReviewStateByEventId,
  projectSchedulePeriod,
} from "./projection";

describe("schedule projection", () => {
  const categories = [
    {
      id: 10,
      name: "Work",
      description: "",
      key: "Work",
      color: "#0EA5E9",
      isDefaultGap: false,
      archived: false,
      inUse: false,
    },
  ] as const;

  const tzSegments = [
    {
      id: 1,
      periodId: 1,
      effectiveFromDate: "2026-06-01",
      ianaTz: "America/Vancouver",
    },
  ] as const;

  it("attaches review state and interaction flags to projected items", () => {
    const projected = projectSchedulePeriod({
      events: [
        {
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
        } as never,
      ],
      eventCategoryOverlays: [],
      timeEntries: [
        {
          id: 21,
          periodId: 1,
          localWorkDate: "2026-06-09",
          start: "2026-06-09T18:00:00Z",
          end: "2026-06-09T19:15:00Z",
          durationMinutes: 75,
          categoryId: 10,
          description: "Deep work",
          attestation: "confirmed",
        } as never,
      ],
      gapTimeline: [],
      reviewDecisions: [
        {
          id: 12,
          kind: "new_in_gap",
          eventId: 34,
          tag: "Conflict",
          title: "Focus",
          description: "Needs review",
          actions: [],
        },
      ],
      tzSegments: [...tzSegments],
      categories: [...categories],
      visibleDays: new Set(["2026-06-09"]),
      draftPlacements: {},
    });

    const reviewItem = projected.items.find((item) => item.id === "event-34");
    expect(reviewItem?.metadata).toMatchObject({
      kind: "review",
      opensReviewQueue: true,
      mutable: false,
      excludable: false,
    });

    const timeEntryItem = projected.items.find((item) => item.id === "time-entry-21");
    expect(timeEntryItem?.metadata).toMatchObject({
      kind: "manual",
      mutable: true,
      excludable: false,
      opensReviewQueue: false,
    });

    expect(projected.reviewDecisionsByEventId.get(34)).toEqual({
      reviewItemId: 12,
      kind: "new_in_gap",
    });
    expect(projected.resettableDays.has("2026-06-09")).toBe(true);
    expect(projected.timeEntriesByItemId.get("time-entry-21")?.id).toBe(21);
  });

  it("hides timed calendar events covered by live calendar-linked drafts", () => {
    const projected = projectSchedulePeriod({
      events: [
        {
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
        } as never,
      ],
      eventCategoryOverlays: [],
      timeEntries: [
        {
          id: 21,
          periodId: 1,
          localWorkDate: "2026-06-09",
          start: "2026-06-09T16:30:00Z",
          end: "2026-06-09T18:00:00Z",
          durationMinutes: 90,
          categoryId: 10,
          description: "Focus",
          attestation: "draft",
          method: "calendar_import",
        } as never,
      ],
      gapTimeline: [],
      reviewDecisions: [],
      tzSegments: [...tzSegments],
      categories: [...categories],
      visibleDays: new Set(["2026-06-09"]),
      draftPlacements: {},
    });

    expect(projected.items.find((item) => item.id === "event-34")).toBeUndefined();
    expect(projected.items.find((item) => item.id === "time-entry-21")?.metadata).toMatchObject({
      kind: "draft",
    });
  });

  it("resolves archived category names on historical time entries", () => {
    const projected = projectSchedulePeriod({
      events: [],
      eventCategoryOverlays: [],
      timeEntries: [
        {
          id: 21,
          periodId: 1,
          localWorkDate: "2026-06-09",
          start: "2026-06-09T18:00:00Z",
          end: "2026-06-09T19:15:00Z",
          durationMinutes: 75,
          categoryId: 99,
          description: "Old bucket",
          attestation: "confirmed",
        } as never,
      ],
      gapTimeline: [],
      reviewDecisions: [],
      tzSegments: [...tzSegments],
      categories: [
        {
          id: 99,
          name: "Legacy Work",
          description: "",
          key: "Legacy Work",
          color: "#64748B",
          isDefaultGap: false,
          archived: true,
          inUse: true,
        },
      ],
      visibleDays: new Set(["2026-06-09"]),
      draftPlacements: {},
    });

    expect(projected.items.find((item) => item.id === "time-entry-21")?.metadata).toMatchObject({
      category: "Legacy Work",
      categoryColor: "#64748B",
    });
  });

  it("builds resettable days from user-created time entries only", () => {
    expect(
      buildResettableDays([
        { localWorkDate: "2026-07-01" } as never,
        { localWorkDate: "2026-07-02", method: "gap_fill" } as never,
      ]),
    ).toEqual(new Set(["2026-07-01"]));
  });

  it("indexes review decisions by event id", () => {
    expect(
      buildReviewStateByEventId([
        { id: 5, kind: "deleted", eventId: 9 } as never,
        { id: 6, kind: "orphan", eventId: undefined } as never,
      ]).get(9),
    ).toEqual({ reviewItemId: 5, kind: "deleted" });
  });
});
