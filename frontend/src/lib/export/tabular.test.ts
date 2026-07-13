import { describe, expect, it } from "vitest";
import {
  defaultTabularSpec,
  encodeTabularSpec,
  fieldCatalog,
  parseTabularSpec,
} from "./tabular";

describe("tabular export specs", () => {
  it("round-trips default flat rollup spec", () => {
    const spec = defaultTabularSpec("rollup", "flat");
    const encoded = encodeTabularSpec(spec);
    const parsed = parseTabularSpec(encoded);
    expect(parsed).toEqual(spec);
  });

  it("exposes matrix catalog without day fields", () => {
    const fields = fieldCatalog("rollup", "matrix").map((field) => field.field);
    expect(fields).toContain("category_name");
    expect(fields).toContain("total");
    expect(fields).not.toContain("date");
  });

  it("forces detail grain to flat defaults", () => {
    const spec = defaultTabularSpec("detail", "matrix");
    expect(spec.layout).toBe("flat");
    expect(spec.columns.map((column) => column.field)).toContain("start");
    expect(spec.columns.map((column) => column.field)).toContain("description");
  });

  it("exposes description in detail catalog", () => {
    const fields = fieldCatalog("detail", "flat").map((field) => field.field);
    expect(fields).toContain("description");
  });

  it("exposes allocation fields in detail catalog and defaults", () => {
    const fields = fieldCatalog("detail", "flat").map((field) => field.field);
    expect(fields).toEqual(
      expect.arrayContaining([
        "work_type",
        "project_name",
        "project_key",
        "billable_status",
      ]),
    );
    const spec = defaultTabularSpec("detail", "flat");
    expect(spec.columns.map((column) => column.field)).toEqual(
      expect.arrayContaining([
        "work_type",
        "project_name",
        "project_key",
        "billable_status",
      ]),
    );
  });
});
