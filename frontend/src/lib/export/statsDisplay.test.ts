import { describe, expect, it } from "vitest";
import { formatDecimalHours } from "@/lib/export/formatters";
import { sortCategoriesByMinutes } from "@/lib/export/summary";
import { categoryStatColor } from "@/lib/category/colors";
import { workingDaysRemaining } from "@/lib/schedule/date";

describe("formatDecimalHours", () => {
  it("formats whole hours without decimals", () => {
    expect(formatDecimalHours(480)).toBe("8h");
  });

  it("formats fractional hours with one decimal", () => {
    expect(formatDecimalHours(315)).toBe("5.3h");
  });
});

describe("workingDaysRemaining", () => {
  const period = {
    startDate: "2026-06-08",
    endDate: "2026-06-14",
  };

  it("counts weekdays from today through period end", () => {
    expect(workingDaysRemaining(period, "2026-06-10")).toBe(3);
  });

  it("returns zero after the period ends", () => {
    expect(workingDaysRemaining(period, "2026-06-20")).toBe(0);
  });
});

describe("sortCategoriesByMinutes", () => {
  it("sorts categories by minutes descending with alpha tie-break", () => {
    expect(
      sortCategoriesByMinutes({
        Admin: 60,
        Engineering: 240,
        Client: 240,
        Product: 120,
      }),
    ).toEqual(["Client", "Engineering", "Product", "Admin"]);
  });
});

describe("categoryStatColor", () => {
  it("returns the configured palette color for a category", () => {
    expect(
      categoryStatColor("Meetings", { Meetings: "#0EA5E9" }),
    ).toBe("#0EA5E9");
  });

  it("falls back to the default color when missing or invalid", () => {
    expect(categoryStatColor("Unknown", {})).toBe("#64748B");
    expect(categoryStatColor("Unknown", { Unknown: "nope" })).toBe("#64748B");
  });
});
