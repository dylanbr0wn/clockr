import { create } from "@bufbuild/protobuf";
import { describe, expect, it } from "vitest";

import {
  BuildPeriodExportResponseSchema,
  CategorySchema,
  ReviewDecisionSchema,
} from "@/gen/shiet/app/v1/application_pb";
import {
  mapCategory,
  mapPeriodExportModel,
  mapReviewDecision,
} from "./applicationRpc";

describe("application RPC mapping", () => {
  it("maps protobuf identifiers only inside the safe integer range", () => {
    expect(mapCategory(create(CategorySchema, { id: 42n, name: "Work" }))).toMatchObject({
      id: 42,
      name: "Work",
      archived: false,
      inUse: false,
    });
    expect(() =>
      mapCategory(create(CategorySchema, { id: BigInt(Number.MAX_SAFE_INTEGER) + 1n })),
    ).toThrow(/safe integer range/);
  });

  it("checks nested export identifiers before mapping", () => {
    const unsafe = BigInt(Number.MAX_SAFE_INTEGER) + 1n;
    expect(() =>
      mapPeriodExportModel(
        create(BuildPeriodExportResponseSchema, {
          periodId: 1n,
          entries: [{ sourceId: unsafe }],
        }),
      ),
    ).toThrow(/safe integer range/);
  });

  it("rejects unknown wire enum values instead of casting them into UI types", () => {
    expect(() =>
      mapReviewDecision(
        create(ReviewDecisionSchema, {
          id: 1n,
          actions: [{ key: "accept", role: 99 as never }],
        }),
      ),
    ).toThrow(/unknown review action role/);
  });
});
