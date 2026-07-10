import { describe, expect, it } from "vitest";
import type { ReviewDecision } from "@/lib/api";
import { buildAttentionItems } from "@/components/stats/attentionItems";
import type { ScheduleGapOverlay } from "@/lib/schedule";

describe("buildAttentionItems", () => {
  it("groups gaps and deleted review decisions", () => {
    const reviewDecisions: ReviewDecision[] = [
      {
        id: 2,
        kind: "deleted_categorized",
        eventId: 1,
        tag: "Removed",
        title: "Design review",
        description: "Deleted from your calendar",
        actions: [],
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
      reviewDecisions,
      events: [],
      visibleGaps,
    });

    expect(items.map((item) => item.id)).toEqual(["gaps", "deleted"]);
    expect(items[0]).toMatchObject({
      title: "2 unfilled gaps",
      subtitle: "3h uncategorized",
      actionLabel: "Fill",
    });
  });

  it("returns empty when there are no decisions or gaps", () => {
    expect(
      buildAttentionItems({
        reviewDecisions: [],
        events: [],
        visibleGaps: [],
      }),
    ).toEqual([]);
  });
});
