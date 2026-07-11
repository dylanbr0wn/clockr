import { useMemo } from "react";
import {
  useCategories,
  useCurrentPeriod,
  usePeriods,
} from "@/lib/api";
import { defaultTimeZone, localDateKey } from "@/lib/schedule";

/**
 * Warms shared React Query cache on every route (categories, periods,
 * ensureCurrentPeriod). Mount from AppShell so cold-starts on /settings
 * still bootstrap domain data that used to ride on SchedulePage.
 */
export function useAppBootstrap() {
  const today = useMemo(() => localDateKey(), []);
  const currentTimeZone = useMemo(() => defaultTimeZone(), []);

  usePeriods();
  useCurrentPeriod(today, currentTimeZone);
  useCategories();
}
