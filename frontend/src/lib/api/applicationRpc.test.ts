import { create } from "@bufbuild/protobuf";
import { describe, expect, it } from "vitest";

import {
  BuildPeriodExportResponseSchema,
  CategorySchema,
  ProjectSchema,
  ReviewDecisionSchema,
  TimeEntrySchema,
} from "@/gen/shiet/app/v1/application_pb";
import {
  mapCategory,
  mapPeriodExportModel,
  mapProject,
  mapReviewDecision,
  mapTimeEntry,
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

  it("maps project masters with safe identifiers", () => {
    expect(
      mapProject(
        create(ProjectSchema, {
          id: 7n,
          name: "Alpha",
          key: "alpha",
          color: "#4F46E5",
          archived: true,
          inUse: true,
        }),
      ),
    ).toEqual({
      id: 7,
      name: "Alpha",
      key: "alpha",
      color: "#4F46E5",
      archived: true,
      inUse: true,
    });
    expect(() =>
      mapProject(create(ProjectSchema, { id: BigInt(Number.MAX_SAFE_INTEGER) + 1n })),
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

  it("maps draft time entries for confirm/reject responses", () => {
    expect(
      mapTimeEntry(
        create(TimeEntrySchema, {
          id: 9n,
          periodId: 3n,
          localWorkDate: "2026-06-09",
          start: "2026-06-09T17:00:00Z",
          end: "2026-06-09T18:00:00Z",
          durationMinutes: 60,
          attestation: "draft",
          workType: "worked",
          billableStatus: "unset",
          description: "Proposed",
          categoryId: 2n,
        }),
      ),
    ).toEqual({
      id: 9,
      periodId: 3,
      localWorkDate: "2026-06-09",
      start: "2026-06-09T17:00:00Z",
      end: "2026-06-09T18:00:00Z",
      durationMinutes: 60,
      attestation: "draft",
      workType: "worked",
      billableStatus: "unset",
      description: "Proposed",
      categoryId: 2,
    });
  });
});
