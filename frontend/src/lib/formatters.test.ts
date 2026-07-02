import { describe, expect, it } from "vitest";
import { formatSyncResultSummary } from "./formatters";

describe("formatSyncResultSummary", () => {
  it("summarizes non-zero sync counts", () => {
    expect(
      formatSyncResultSummary({
        added: 2,
        updated: 1,
        unchanged: 5,
        removed: 0,
        flagged: 3,
      }),
    ).toBe("2 added, 1 updated, 3 flagged");
  });

  it("falls back to unchanged when nothing else changed", () => {
    expect(
      formatSyncResultSummary({
        added: 0,
        updated: 0,
        unchanged: 4,
        removed: 0,
        flagged: 0,
      }),
    ).toBe("4 unchanged");
  });

  it("reports no changes when everything is zero", () => {
    expect(
      formatSyncResultSummary({
        added: 0,
        updated: 0,
        unchanged: 0,
        removed: 0,
        flagged: 0,
      }),
    ).toBe("No changes");
  });
});
