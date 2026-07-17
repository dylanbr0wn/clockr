import { describe, expect, it } from "vitest";
import { showsProjectAndBillableFields } from "./allocation";

describe("showsProjectAndBillableFields", () => {
  it("hides project and billable for leave break and holiday", () => {
    expect(showsProjectAndBillableFields("paid_leave")).toBe(false);
    expect(showsProjectAndBillableFields("unpaid_leave")).toBe(false);
    expect(showsProjectAndBillableFields("holiday")).toBe(false);
    expect(showsProjectAndBillableFields("break")).toBe(false);
  });

  it("shows project and billable for worked and adjustment", () => {
    expect(showsProjectAndBillableFields("worked")).toBe(true);
    expect(showsProjectAndBillableFields("adjustment")).toBe(true);
  });
});
