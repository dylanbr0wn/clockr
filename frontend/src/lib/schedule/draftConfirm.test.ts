import { describe, expect, it } from "vitest";
import type { TimeEntry } from "@/lib/api";
import {
  OVERNIGHT_ATTRIBUTE_TO_START,
  OVERNIGHT_SPLIT_AT_MIDNIGHT,
  OVERLAP_ALLOW_PARALLEL,
  OVERLAP_KEEP_MINE,
  OVERLAP_KEEP_THEIRS,
  OVERLAP_SPLIT,
  entryCrossesLocalMidnight,
  findOverlappingConfirmed,
  isValidSplitCutPoints,
} from "./draftConfirm";

function entry(
  partial: Pick<TimeEntry, "id" | "start" | "end"> &
    Partial<Omit<TimeEntry, "id" | "start" | "end">>,
): TimeEntry {
  return {
    periodId: 1,
    localWorkDate: "2026-06-09",
    durationMinutes: 60,
    attestation: "confirmed",
    workType: "worked",
    billableStatus: "unset",
    ...partial,
  };
}

describe("draftConfirm policy constants", () => {
  it("matches backend overnight and overlap string values", () => {
    expect(OVERNIGHT_ATTRIBUTE_TO_START).toBe("attribute_to_start");
    expect(OVERNIGHT_SPLIT_AT_MIDNIGHT).toBe("split_at_midnight");
    expect(OVERLAP_KEEP_MINE).toBe("keep_mine");
    expect(OVERLAP_KEEP_THEIRS).toBe("keep_theirs");
    expect(OVERLAP_SPLIT).toBe("split");
    expect(OVERLAP_ALLOW_PARALLEL).toBe("allow_parallel");
  });
});

describe("entryCrossesLocalMidnight", () => {
  it("is false when start and end share a local calendar day", () => {
    expect(
      entryCrossesLocalMidnight(
        "2026-06-09T16:00:00.000Z",
        "2026-06-09T18:00:00.000Z",
        "America/Los_Angeles",
      ),
    ).toBe(false);
  });

  it("is true when local calendar day of end differs from start", () => {
    // 10pm–2am Pacific on 2026-06-09/10
    expect(
      entryCrossesLocalMidnight(
        "2026-06-10T05:00:00.000Z",
        "2026-06-10T09:00:00.000Z",
        "America/Los_Angeles",
      ),
    ).toBe(true);
  });

  it("is false for invalid timestamps", () => {
    expect(
      entryCrossesLocalMidnight("not-a-date", "2026-06-09T18:00:00.000Z", "UTC"),
    ).toBe(false);
  });
});

describe("findOverlappingConfirmed", () => {
  const draft = { id: 10, start: "2026-06-09T17:00:00.000Z", end: "2026-06-09T19:00:00.000Z" };

  it("returns confirmed entries that overlap the draft interval", () => {
    const confirmed = [
      entry({ id: 1, start: "2026-06-09T16:00:00.000Z", end: "2026-06-09T17:30:00.000Z" }),
      entry({ id: 2, start: "2026-06-09T19:00:00.000Z", end: "2026-06-09T20:00:00.000Z" }),
      entry({ id: 3, start: "2026-06-09T18:00:00.000Z", end: "2026-06-09T18:30:00.000Z" }),
    ];

    expect(findOverlappingConfirmed(draft, confirmed).map((e) => e.id)).toEqual([
      1, 3,
    ]);
  });

  it("excludes self by id and non-confirmed attestations", () => {
    const confirmed = [
      entry({
        id: 10,
        start: "2026-06-09T17:00:00.000Z",
        end: "2026-06-09T19:00:00.000Z",
        attestation: "confirmed",
      }),
      entry({
        id: 11,
        start: "2026-06-09T17:30:00.000Z",
        end: "2026-06-09T18:00:00.000Z",
        attestation: "draft",
      }),
      entry({
        id: 12,
        start: "2026-06-09T17:30:00.000Z",
        end: "2026-06-09T18:00:00.000Z",
        attestation: "confirmed",
      }),
    ];

    expect(findOverlappingConfirmed(draft, confirmed).map((e) => e.id)).toEqual([
      12,
    ]);
  });
});

describe("isValidSplitCutPoints", () => {
  const start = "2026-06-09T17:00:00.000Z";
  const end = "2026-06-09T20:00:00.000Z";

  it("accepts ascending cut points strictly inside the span", () => {
    expect(
      isValidSplitCutPoints(start, end, [
        "2026-06-09T18:00:00.000Z",
        "2026-06-09T19:00:00.000Z",
      ]),
    ).toBe(true);
  });

  it("rejects empty, unsorted, boundary, or invalid cuts", () => {
    expect(isValidSplitCutPoints(start, end, [])).toBe(false);
    expect(
      isValidSplitCutPoints(start, end, [
        "2026-06-09T19:00:00.000Z",
        "2026-06-09T18:00:00.000Z",
      ]),
    ).toBe(false);
    expect(isValidSplitCutPoints(start, end, [start])).toBe(false);
    expect(isValidSplitCutPoints(start, end, [end])).toBe(false);
    expect(isValidSplitCutPoints(start, end, ["nope"])).toBe(false);
  });
});
