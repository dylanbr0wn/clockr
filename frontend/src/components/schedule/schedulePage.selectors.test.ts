import { describe, expect, it } from "vitest";
import { buildSchedulePageDerived, resolveActivePeriod } from "./schedulePage.selectors";

describe("schedulePage.selectors", () => {
  it("resolves active period by precedence", () => {
    const periods = [
      { id: 1, startDate: "2026-07-01", endDate: "2026-07-14" },
      { id: 2, startDate: "2026-07-15", endDate: "2026-07-28" },
    ];
    const current = { id: 2, startDate: "2026-07-15", endDate: "2026-07-28" };

    const selected = resolveActivePeriod({
      selectedPeriodId: 1,
      currentPeriod: current as never,
      periods: periods as never,
      today: "2026-07-10",
    });
    expect(selected?.id).toBe(1);

    const fallbackToCurrent = resolveActivePeriod({
      selectedPeriodId: null,
      currentPeriod: current as never,
      periods: periods as never,
      today: "2026-07-10",
    });
    expect(fallbackToCurrent?.id).toBe(2);
  });

  it("builds totals and resettable days from input data", () => {
    const derived = buildSchedulePageDerived({
      selectedPeriodId: 1,
      viewDayCount: 7,
      today: "2026-07-01",
      persistedPeriods: [
        { id: 1, startDate: "2026-07-01", endDate: "2026-07-14" } as never,
      ],
      currentPeriod: null,
      categories: [{ id: 10, name: "Work", color: "#000000" } as never],
      events: [],
      gapFills: [
        {
          id: 7,
          periodId: 1,
          day: "2026-07-01",
          start: "2026-07-01T09:00:00Z",
          end: "2026-07-01T10:00:00Z",
          startMinutes: 540,
          endMinutes: 600,
          categoryId: 10,
          note: "focus",
          source: "manual",
        } as never,
      ],
      gapTimeline: [],
      reviewItems: [],
      tzSegments: [],
      draftPlacements: {},
      pendingCreate: null,
      editingItemId: null,
    });

    expect(derived.activePeriodId).toBe(1);
    expect(derived.resettableDays.has("2026-07-01")).toBe(true);
    expect(derived.totals.Work).toBe(60);
  });
});
