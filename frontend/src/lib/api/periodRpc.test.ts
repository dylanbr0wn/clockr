import { describe, expect, it } from "vitest";

import type { Period as WirePeriod } from "@/gen/shiet/app/v1/period_pb";
import { mapPeriod } from "./periodRpc";

describe("mapPeriod", () => {
  it("maps protobuf bigint identifiers without leaking wire types", () => {
    const result = mapPeriod({
      id: 42n,
      startDate: "2026-07-06",
      endDate: "2026-07-19",
      cadence: "bi-weekly",
      anchorDate: "2026-07-06",
      targetHoursPerDay: 8,
    } as WirePeriod);

    expect(result).toEqual({
      id: 42,
      startDate: "2026-07-06",
      endDate: "2026-07-19",
      cadence: "bi-weekly",
      anchorDate: "2026-07-06",
      targetHoursPerDay: 8,
    });
  });

  it("rejects identifiers outside JavaScript's safe integer range", () => {
    expect(() => mapPeriod({ id: BigInt(Number.MAX_SAFE_INTEGER) + 1n } as WirePeriod)).toThrow(
      "outside JavaScript's safe integer range",
    );
  });
});
