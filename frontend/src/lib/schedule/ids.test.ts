import { describe, expect, it } from "vitest";
import {
  allDayChipId,
  buildGapFillsByItemId,
  eventItemId,
  gapFillItemId,
  isEventItemId,
  isGapFillItemId,
  parseEventItemId,
  parseGapFillItemId,
} from "./ids";

describe("schedule ids", () => {
  it("round-trips event and gap-fill item ids", () => {
    expect(eventItemId(34)).toBe("event-34");
    expect(parseEventItemId("event-34")).toBe(34);
    expect(isEventItemId("event-34")).toBe(true);
    expect(isEventItemId("gap-fill-34")).toBe(false);

    expect(gapFillItemId(21)).toBe("gap-fill-21");
    expect(parseGapFillItemId("gap-fill-21")).toBe(21);
    expect(isGapFillItemId("gap-fill-21")).toBe(true);
    expect(isGapFillItemId("event-21")).toBe(false);
  });

  it("builds all-day chip ids and gap-fill lookup maps", () => {
    expect(allDayChipId(7, "2026-07-03")).toBe("event-7@2026-07-03");

    const gapFills = [
      {
        id: 11,
        periodId: 1,
        day: "2026-07-02",
        start: "2026-07-02T09:00:00Z",
        end: "2026-07-02T10:00:00Z",
        categoryId: 10,
        note: "",
        source: "manual",
      },
    ] as const;

    const byItemId = buildGapFillsByItemId([...gapFills]);
    expect(byItemId.get("gap-fill-11")).toEqual(gapFills[0]);
  });
});
