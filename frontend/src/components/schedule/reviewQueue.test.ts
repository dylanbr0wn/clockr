import { describe, expect, it } from "vitest";
import { kindBadgeClass } from "./reviewQueue";

describe("kindBadgeClass", () => {
  it("returns styling classes for known review kinds", () => {
    expect(kindBadgeClass("deleted_categorized")).toContain("destructive");
    expect(kindBadgeClass("new_in_gap")).toContain("amber");
    expect(kindBadgeClass("source_drift")).toContain("orange");
    expect(kindBadgeClass("unknown_kind")).toContain("muted");
  });
});
