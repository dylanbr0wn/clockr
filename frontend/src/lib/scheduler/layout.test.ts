import { describe, expect, it } from "vitest";
import { packOverlaps } from "./layout";
import type { SchedulerItem, SchedulerVisibleRange } from "./types";

const range: SchedulerVisibleRange = {
  startMinutes: 8 * 60,
  endMinutes: 18 * 60,
};

function item(id: string, startMinutes: number, endMinutes: number): SchedulerItem {
  return { id, day: "2026-06-08", startMinutes, endMinutes };
}

describe("packOverlaps", () => {
  it("gives non-overlapping items the full width", () => {
    const layout = packOverlaps(
      [item("a", 9 * 60, 10 * 60), item("b", 10 * 60, 11 * 60)],
      range,
    );

    expect(layout).toHaveLength(2);
    for (const entry of layout) {
      expect(entry.widthPercent).toBe(100);
      expect(entry.laneCount).toBe(1);
      expect(entry.overlaps).toBe(false);
    }
  });

  it("splits overlapping items into lanes", () => {
    const layout = packOverlaps(
      [item("a", 9 * 60, 10 * 60), item("b", 9 * 60 + 30, 10 * 60 + 30)],
      range,
    );

    const [a, b] = layout;
    expect(a.lane).toBe(0);
    expect(b.lane).toBe(1);
    expect(a.widthPercent).toBe(50);
    expect(b.leftPercent).toBe(50);
    expect(a.overlaps).toBe(true);
    expect(b.overlaps).toBe(true);
  });

  it("reuses freed lanes within a group", () => {
    // a: 9-10, b: 9:30-10:30, c: 10-11 — c fits back into lane 0
    const layout = packOverlaps(
      [
        item("a", 9 * 60, 10 * 60),
        item("b", 9 * 60 + 30, 10 * 60 + 30),
        item("c", 10 * 60, 11 * 60),
      ],
      range,
    );

    const byId = Object.fromEntries(layout.map((entry) => [entry.item.id, entry]));
    expect(byId.a.lane).toBe(0);
    expect(byId.b.lane).toBe(1);
    expect(byId.c.lane).toBe(0);
    expect(byId.c.laneCount).toBe(2);
    // c only overlaps b, not a
    expect(byId.c.overlaps).toBe(true);
  });

  it("computes percent geometry against the visible range", () => {
    const [entry] = packOverlaps([item("a", 9 * 60, 10 * 60)], range);
    expect(entry.topPercent).toBe(10);
    expect(entry.heightPercent).toBe(10);
  });

  it("flags the preview item", () => {
    const layout = packOverlaps(
      [item("a", 9 * 60, 10 * 60), item("b", 12 * 60, 13 * 60)],
      range,
      "b",
    );
    const byId = Object.fromEntries(layout.map((entry) => [entry.item.id, entry]));
    expect(byId.a.isPreview).toBe(false);
    expect(byId.b.isPreview).toBe(true);
  });
});
