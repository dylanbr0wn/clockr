import { describe, expect, it } from "vitest";
import {
  MINUTES_PER_DAY,
  addDays,
  calculateVisibleRange,
  formatMinutes,
  minutesToPercent,
  normalizeConfig,
  snapMinutes,
  snapMinutesDown,
  snapMinutesUp,
} from "./time";

describe("addDays", () => {
  it("adds days within a month", () => {
    expect(addDays("2026-06-08", 1)).toBe("2026-06-09");
    expect(addDays("2026-06-08", 6)).toBe("2026-06-14");
  });

  it("rolls over month and year boundaries", () => {
    expect(addDays("2026-06-30", 1)).toBe("2026-07-01");
    expect(addDays("2026-12-31", 1)).toBe("2027-01-01");
    expect(addDays("2024-02-28", 1)).toBe("2024-02-29");
  });

  it("supports zero and negative offsets", () => {
    expect(addDays("2026-06-08", 0)).toBe("2026-06-08");
    expect(addDays("2026-06-01", -1)).toBe("2026-05-31");
  });
});

describe("snapping", () => {
  it("rounds to the nearest slot", () => {
    expect(snapMinutes(472, 15)).toBe(465);
    expect(snapMinutes(473, 15)).toBe(480);
  });

  it("clamps to the day", () => {
    expect(snapMinutes(-30, 15)).toBe(0);
    expect(snapMinutes(MINUTES_PER_DAY + 30, 15)).toBe(MINUTES_PER_DAY);
  });

  it("floors and ceils", () => {
    expect(snapMinutesDown(473, 15)).toBe(465);
    expect(snapMinutesUp(466, 15)).toBe(480);
  });
});

describe("normalizeConfig", () => {
  it("applies defaults", () => {
    const config = normalizeConfig();
    expect(config.slotMinutes).toBe(15);
    expect(config.scheduleStartMinutes).toBe(0);
    expect(config.scheduleEndMinutes).toBe(MINUTES_PER_DAY);
    expect(config.workingStartMinutes).toBe(8 * 60);
  });

  it("sanitizes degenerate values", () => {
    const config = normalizeConfig({
      slotMinutes: 0,
      minDurationMinutes: -5,
      maxDays: 0,
      dragThresholdPx: -1,
      scheduleStartMinutes: MINUTES_PER_DAY + 60,
      scheduleEndMinutes: 0,
    });
    expect(config.slotMinutes).toBe(1);
    expect(config.minDurationMinutes).toBe(1);
    expect(config.maxDays).toBe(1);
    expect(config.dragThresholdPx).toBe(0);
    expect(config.scheduleStartMinutes).toBe(MINUTES_PER_DAY - 1);
    expect(config.scheduleEndMinutes).toBe(MINUTES_PER_DAY);
  });
});

describe("calculateVisibleRange", () => {
  const config = normalizeConfig({
    scheduleStartMinutes: 6 * 60,
    scheduleEndMinutes: 20 * 60,
  });

  it("falls back to the configured schedule range with no items", () => {
    expect(calculateVisibleRange([], config)).toEqual({
      startMinutes: 6 * 60,
      endMinutes: 20 * 60,
    });
  });

  it("expands without clipping off-slot items", () => {
    const range = calculateVisibleRange(
      [
        { id: "a", day: "2026-06-08", startMinutes: 5 * 60 + 53, endMinutes: 9 * 60 },
        { id: "b", day: "2026-06-08", startMinutes: 17 * 60, endMinutes: 20 * 60 + 7 },
      ],
      config,
    );
    // floor start / ceil end to the slot grid so both items stay fully visible
    expect(range.startMinutes).toBe(5 * 60 + 45);
    expect(range.endMinutes).toBe(20 * 60 + 15);
  });

  it("clamps to the day", () => {
    const range = calculateVisibleRange(
      [{ id: "a", day: "2026-06-08", startMinutes: 0, endMinutes: MINUTES_PER_DAY }],
      config,
    );
    expect(range.startMinutes).toBe(0);
    expect(range.endMinutes).toBe(MINUTES_PER_DAY);
  });
});

describe("percent helpers", () => {
  const range = { startMinutes: 8 * 60, endMinutes: 18 * 60 };

  it("maps minutes to percent of the visible range", () => {
    expect(minutesToPercent(8 * 60, range)).toBe(0);
    expect(minutesToPercent(13 * 60, range)).toBe(50);
    expect(minutesToPercent(18 * 60, range)).toBe(100);
  });

  it("handles empty ranges", () => {
    expect(minutesToPercent(60, { startMinutes: 60, endMinutes: 60 })).toBe(0);
  });
});

describe("formatMinutes", () => {
  it("formats zero-padded HH:MM", () => {
    expect(formatMinutes(0)).toBe("00:00");
    expect(formatMinutes(9 * 60 + 5)).toBe("09:05");
    expect(formatMinutes(23 * 60 + 59)).toBe("23:59");
  });
});
