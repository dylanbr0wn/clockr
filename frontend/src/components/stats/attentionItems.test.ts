import { describe, expect, it } from "vitest";
import type { Event, ReviewItem } from "@/lib/api";
import { buildAttentionItems } from "@/components/stats/attentionItems";
import type { ScheduleGapOverlay } from "@/lib/schedule";

const event: Event = {
  id: 1,
  periodId: 1,
  calendarId: 1,
  provider: "google",
  externalId: "evt-1",
  title: "Sprint planning",
  allDay: false,
  active: true,
};

describe("buildAttentionItems", () => {
  it("groups overlaps, gaps, and deleted review items", () => {
    const reviewItems: ReviewItem[] = [
      {
        id: 1,
        periodId: 1,
        kind: "overlap",
        eventId: 1,
        payload: JSON.stringify({ title: "Sprint planning" }),
        status: "open",
      },
      {
        id: 2,
        periodId: 1,
        kind: "deleted_categorized",
        eventId: 1,
        payload: JSON.stringify({ title: "Design review" }),
        status: "open",
      },
    ];
    const visibleGaps: ScheduleGapOverlay[] = [
      {
        id: "gap-1",
        day: "2026-06-09",
        startMinutes: 10 * 60,
        endMinutes: 12 * 60,
        gapWindowStart: "2026-06-09T10:00:00Z",
        gapWindowEnd: "2026-06-09T12:00:00Z",
      },
      {
        id: "gap-2",
        day: "2026-06-12",
        startMinutes: 11 * 60,
        endMinutes: 12 * 60,
        gapWindowStart: "2026-06-12T11:00:00Z",
        gapWindowEnd: "2026-06-12T12:00:00Z",
      },
    ];

    const items = buildAttentionItems({
      reviewItems,
      events: [event],
      visibleGaps,
    });

    expect(items.map((item) => item.id)).toEqual(["overlap", "gaps", "deleted"]);
    expect(items[1]).toMatchObject({
      title: "2 unfilled gaps",
      subtitle: "3h uncategorized",
      actionLabel: "Fill",
    });
  });

  it("ignores closed review items", () => {
    const reviewItems: ReviewItem[] = [
      {
        id: 1,
        periodId: 1,
        kind: "deleted_categorized",
        eventId: 1,
        payload: "{}",
        status: "resolved",
      },
    ];

    expect(
      buildAttentionItems({
        reviewItems,
        events: [event],
        visibleGaps: [],
      }),
    ).toEqual([]);
  });
});
