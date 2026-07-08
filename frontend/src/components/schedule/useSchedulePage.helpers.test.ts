import { describe, expect, it } from "vitest";
import {
  anyLoading,
  calculateTotals,
  firstError,
  mergePeriods,
} from "./useSchedulePage.helpers";

describe("useSchedulePage.helpers", () => {
  it("merges current period when not present", () => {
    const persisted = [
      { id: 2, startDate: "2026-07-01", endDate: "2026-07-14" },
      { id: 1, startDate: "2026-06-17", endDate: "2026-06-30" },
    ];
    const current = { id: 3, startDate: "2026-07-15", endDate: "2026-07-28" };

    const result = mergePeriods(persisted as never, current as never);

    expect(result.map((period) => period.id)).toEqual([3, 2, 1]);
  });

  it("keeps period list unchanged when current already exists", () => {
    const persisted = [{ id: 2 }, { id: 1 }];
    const current = { id: 2 };

    const result = mergePeriods(persisted as never, current as never);

    expect(result).toBe(persisted);
  });

  it("calculates category totals from scheduler items", () => {
    const items = [
      {
        startMinutes: 60,
        endMinutes: 120,
        metadata: { category: "Work" },
      },
      {
        startMinutes: 120,
        endMinutes: 180,
        metadata: { category: "Work" },
      },
      {
        startMinutes: 180,
        endMinutes: 210,
        metadata: {},
      },
    ];

    const totals = calculateTotals(items as never);

    expect(totals).toEqual({
      Work: 120,
      Unassigned: 30,
    });
  });

  it("rolls up loading and error flags", () => {
    expect(anyLoading([false, false, false])).toBe(false);
    expect(anyLoading([false, true, false])).toBe(true);

    const expected = new Error("boom");
    expect(firstError([null, undefined, expected, new Error("later")])).toBe(
      expected,
    );
  });
});
