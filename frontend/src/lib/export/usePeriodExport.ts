import { useMemo } from "react";
import {
  useCategories,
  useEvents,
  useGapFills,
  useTzSegments,
  type Period,
} from "@/lib/api";
import {
  eventToSchedulerItem,
  gapFillToSchedulerItem,
} from "@/lib/schedule";
import { buildPeriodExportSummary } from "./summary";

export function usePeriodExport(period: Period | null | undefined) {
  const periodId = period?.id;
  const eventsQuery = useEvents(periodId);
  const gapFillsQuery = useGapFills(periodId);
  const tzSegmentsQuery = useTzSegments(periodId);
  const categoriesQuery = useCategories();

  const categoriesById = useMemo(() => {
    return new Map(
      (categoriesQuery.data ?? []).map((category) => [category.id, category]),
    );
  }, [categoriesQuery.data]);

  const items = useMemo(() => {
    const events = eventsQuery.data ?? [];
    const gapFills = gapFillsQuery.data ?? [];
    const tzSegments = tzSegmentsQuery.data ?? [];

    return [
      ...events
        .map((event) => eventToSchedulerItem(event, tzSegments))
        .filter((item): item is NonNullable<typeof item> => item !== null),
      ...gapFills
        .map((gapFill) =>
          gapFillToSchedulerItem(gapFill, categoriesById, tzSegments),
        )
        .filter((item): item is NonNullable<typeof item> => item !== null),
    ];
  }, [
    categoriesById,
    eventsQuery.data,
    gapFillsQuery.data,
    tzSegmentsQuery.data,
  ]);

  const summary = useMemo(() => {
    if (!period) {
      return null;
    }

    return buildPeriodExportSummary(items, period);
  }, [items, period]);

  const totals = useMemo(() => {
    return items.reduce<Record<string, number>>((next, item) => {
      const key = item.metadata?.category ?? "Unassigned";
      next[key] = (next[key] ?? 0) + item.endMinutes - item.startMinutes;
      return next;
    }, {});
  }, [items]);

  const isLoading =
    eventsQuery.isLoading ||
    gapFillsQuery.isLoading ||
    tzSegmentsQuery.isLoading ||
    categoriesQuery.isLoading;
  const error =
    eventsQuery.error ??
    gapFillsQuery.error ??
    tzSegmentsQuery.error ??
    categoriesQuery.error;

  return {
    items,
    summary,
    totals,
    isLoading,
    error,
  };
}
