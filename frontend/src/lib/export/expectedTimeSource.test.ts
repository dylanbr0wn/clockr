import { describe, expect, it } from "vitest";
import { formatExpectedTimeSourceLabel } from "./expectedTimeSource";

describe("formatExpectedTimeSourceLabel", () => {
  it("labels weekday template source", () => {
    expect(formatExpectedTimeSourceLabel("weekday")).toBe("Weekday template");
  });

  it("labels holiday exception", () => {
    expect(formatExpectedTimeSourceLabel("exception", "holiday")).toBe(
      "Holiday",
    );
  });

  it("labels leave exception", () => {
    expect(formatExpectedTimeSourceLabel("exception", "leave")).toBe("Leave");
  });

  it("labels changed-hours exception", () => {
    expect(formatExpectedTimeSourceLabel("exception", "changed_hours")).toBe(
      "Changed hours",
    );
  });

  it("returns empty for unknown source", () => {
    expect(formatExpectedTimeSourceLabel("")).toBe("");
    expect(formatExpectedTimeSourceLabel("other")).toBe("");
  });

  it("returns empty for exception without a known kind", () => {
    expect(formatExpectedTimeSourceLabel("exception")).toBe("");
    expect(formatExpectedTimeSourceLabel("exception", "pto")).toBe("");
  });
});
