import { createClient, type Client } from "@connectrpc/connect";

import {
  PeriodService,
  type Period as WirePeriod,
} from "@/gen/shiet/app/v1/period_pb";
import type { Period } from "./types";
import { rpcTransport } from "./rpcTransport";

let client: Client<typeof PeriodService> | undefined;

function periodClient() {
  client ??= createClient(
    PeriodService,
    rpcTransport(),
  );
  return client;
}

export async function listPeriodsRPC() {
  const response = await periodClient().listPeriods({});
  return response.periods.map(mapPeriod);
}

export async function getPeriodRPC(id: number) {
  const response = await periodClient().getPeriod({ id: BigInt(id) });
  if (!response.period) throw new Error("get period response is missing period");
  return mapPeriod(response.period);
}

export async function getPeriodByRangeRPC(startDate: string, endDate: string) {
  const response = await periodClient().getPeriodByRange({ startDate, endDate });
  if (!response.period) throw new Error("get period by range response is missing period");
  return mapPeriod(response.period);
}

export async function ensureCurrentPeriodRPC(today: string, ianaTz: string) {
  const response = await periodClient().ensureCurrentPeriod({ today, ianaTz });
  return response.period ? mapPeriod(response.period) : null;
}

export function mapPeriod(period: WirePeriod): Period {
  const id = Number(period.id);
  if (!Number.isSafeInteger(id)) {
    throw new Error(`period id ${period.id} is outside JavaScript's safe integer range`);
  }
  return {
    id,
    startDate: period.startDate,
    endDate: period.endDate,
    cadence: period.cadence,
    anchorDate: period.anchorDate,
    targetHoursPerDay: period.targetHoursPerDay,
    ...(period.lastSyncedAt
      ? {
          lastSyncedAt: new Date(
            Number(period.lastSyncedAt.seconds) * 1_000 +
              period.lastSyncedAt.nanos / 1_000_000,
          ).toISOString(),
        }
      : {}),
  };
}
