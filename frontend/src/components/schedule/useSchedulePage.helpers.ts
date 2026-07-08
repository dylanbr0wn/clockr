import type { Period } from "@/lib/api";
import type { ScheduleItem } from "@/lib/schedule";

export function mergePeriods(
  persistedPeriods: Period[],
  currentPeriod: Period | null,
): Period[] {
  if (
    currentPeriod &&
    !persistedPeriods.some((period) => period.id === currentPeriod.id)
  ) {
    return [currentPeriod, ...persistedPeriods];
  }

  return persistedPeriods;
}

export function calculateTotals(items: ScheduleItem[]): Record<string, number> {
  return items.reduce<Record<string, number>>((next, item) => {
    const key = item.metadata?.category ?? "Unassigned";
    next[key] = (next[key] ?? 0) + item.endMinutes - item.startMinutes;
    return next;
  }, {});
}

export function anyLoading(flags: boolean[]): boolean {
  return flags.some(Boolean);
}

export function firstError(errors: unknown[]): unknown {
  return errors.find((error) => error != null);
}
