import { describe, expect, it } from "vitest";
import {
  CATEGORY_PALETTE,
  categoryColorStyle,
  isCategoryPaletteColor,
} from "./colors";

describe("category colors", () => {
  it("accepts preset palette values case-insensitively", () => {
    expect(isCategoryPaletteColor("#0ea5e9")).toBe(true);
    expect(isCategoryPaletteColor("#123456")).toBe(false);
    expect(CATEGORY_PALETTE).toHaveLength(8);
  });

  it("builds rgba background from palette hex", () => {
    expect(categoryColorStyle("#0EA5E9")).toEqual({
      borderColor: "#0EA5E9",
      backgroundColor: "#0EA5E926",
      color: "var(--foreground)",
    });
  });

  it("falls back to slate for unknown colors", () => {
    expect(categoryColorStyle("nope").borderColor).toBe("#64748B");
  });
});
