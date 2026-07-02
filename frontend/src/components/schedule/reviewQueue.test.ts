import { describe, expect, it } from "vitest";
import type { Event, ReviewItem } from "@/lib/api";
import { buildReviewItemView } from "./reviewQueue";

describe("buildReviewItemView", () => {
  const event: Event = {
    id: 7,
    periodId: 1,
    calendarId: 1,
    provider: "google",
    externalId: "evt-1",
    title: "Standup",
    allDay: false,
    start: "2026-06-02T09:00:00Z",
    end: "2026-06-02T09:30:00Z",
    active: true,
  };

  it("maps deleted categorized items to drop/keep actions", () => {
    const item: ReviewItem = {
      id: 1,
      periodId: 1,
      kind: "deleted_categorized",
      eventId: 7,
      payload: JSON.stringify({ reason: "deleted", title: "Standup" }),
      status: "open",
    };

    const view = buildReviewItemView(item, new Map([[7, event]]));

    expect(view).toMatchObject({
      tag: "Removed",
      primaryAction: { action: "drop_entry" },
      secondaryAction: { action: "keep_entry" },
    });
  });

  it("maps gap conflicts to shrink/keep actions", () => {
    const item: ReviewItem = {
      id: 2,
      periodId: 1,
      kind: "new_in_gap",
      eventId: 7,
      payload: JSON.stringify({ title: "Standup" }),
      status: "open",
    };

    const view = buildReviewItemView(item, new Map([[7, event]]));

    expect(view).toMatchObject({
      tag: "Gap conflict",
      primaryAction: { action: "use_event" },
      secondaryAction: { action: "keep_gap" },
    });
  });
});
