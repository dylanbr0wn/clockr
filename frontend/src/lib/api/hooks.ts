import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  computeGaps,
  createManualEvent,
  ensureCurrentPeriod,
  getSetting,
  listCalendars,
  listCategories,
  listEvents,
  listGapFills,
  listOpenReviewItems,
  listPeriods,
  listSelectedCalendars,
  listTzSegments,
  updateManualEvent,
} from "./clockrService";

export const clockrQueryKeys = {
  all: ["clockr"] as const,
  calendars: () => [...clockrQueryKeys.all, "calendars"] as const,
  categories: () => [...clockrQueryKeys.all, "categories"] as const,
  gapTimeline: (periodId: number) =>
    [...clockrQueryKeys.period(periodId), "gapTimeline"] as const,
  period: (periodId: number) =>
    [...clockrQueryKeys.periods(), periodId] as const,
  currentPeriod: (today: string, ianaTz: string) =>
    [...clockrQueryKeys.periods(), "current", today, ianaTz] as const,
  periods: () => [...clockrQueryKeys.all, "periods"] as const,
  periodEvents: (periodId: number) =>
    [...clockrQueryKeys.period(periodId), "events"] as const,
  periodGapFills: (periodId: number) =>
    [...clockrQueryKeys.period(periodId), "gapFills"] as const,
  periodReviewItems: (periodId: number) =>
    [...clockrQueryKeys.period(periodId), "reviewItems"] as const,
  periodTzSegments: (periodId: number) =>
    [...clockrQueryKeys.period(periodId), "tzSegments"] as const,
  selectedCalendars: () =>
    [...clockrQueryKeys.calendars(), "selected"] as const,
  setting: (key: string) => [...clockrQueryKeys.all, "settings", key] as const,
};

export function usePeriods() {
  return useQuery({
    queryKey: clockrQueryKeys.periods(),
    queryFn: listPeriods,
  });
}

export function useCurrentPeriod(today: string, ianaTz: string) {
  return useQuery({
    queryKey: clockrQueryKeys.currentPeriod(today, ianaTz),
    queryFn: () => ensureCurrentPeriod(today, ianaTz),
  });
}

export function useCategories() {
  return useQuery({
    queryKey: clockrQueryKeys.categories(),
    queryFn: listCategories,
  });
}

export function useCalendars() {
  return useQuery({
    queryKey: clockrQueryKeys.calendars(),
    queryFn: listCalendars,
  });
}

export function useSelectedCalendars() {
  return useQuery({
    queryKey: clockrQueryKeys.selectedCalendars(),
    queryFn: listSelectedCalendars,
  });
}

export function useEvents(periodId: number | null | undefined) {
  return useQuery({
    enabled: typeof periodId === "number",
    queryKey: clockrQueryKeys.periodEvents(periodId ?? 0),
    queryFn: () => listEvents(periodId as number),
  });
}

export function useGapFills(periodId: number | null | undefined) {
  return useQuery({
    enabled: typeof periodId === "number",
    queryKey: clockrQueryKeys.periodGapFills(periodId ?? 0),
    queryFn: () => listGapFills(periodId as number),
  });
}

export function useCreateManualEvent() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: createManualEvent,
    onSuccess: (gapFill, input) => {
      const periodId = gapFill.periodId || input.periodId;

      void queryClient.invalidateQueries({
        queryKey: clockrQueryKeys.periodGapFills(periodId),
      });
      void queryClient.invalidateQueries({
        queryKey: clockrQueryKeys.gapTimeline(periodId),
      });
    },
  });
}

export function useUpdateManualEvent() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: updateManualEvent,
    onSuccess: (gapFill, input) => {
      const periodId = gapFill.periodId || input.periodId;

      void queryClient.invalidateQueries({
        queryKey: clockrQueryKeys.periodGapFills(periodId),
      });
      void queryClient.invalidateQueries({
        queryKey: clockrQueryKeys.gapTimeline(periodId),
      });
    },
  });
}

export function useOpenReviewItems(periodId: number | null | undefined) {
  return useQuery({
    enabled: typeof periodId === "number",
    queryKey: clockrQueryKeys.periodReviewItems(periodId ?? 0),
    queryFn: () => listOpenReviewItems(periodId as number),
  });
}

export function useTzSegments(periodId: number | null | undefined) {
  return useQuery({
    enabled: typeof periodId === "number",
    queryKey: clockrQueryKeys.periodTzSegments(periodId ?? 0),
    queryFn: () => listTzSegments(periodId as number),
  });
}

export function useGapTimeline(periodId: number | null | undefined) {
  return useQuery({
    enabled: typeof periodId === "number",
    queryKey: clockrQueryKeys.gapTimeline(periodId ?? 0),
    queryFn: () => computeGaps(periodId as number),
  });
}

export function useSetting(key: string) {
  return useQuery({
    queryKey: clockrQueryKeys.setting(key),
    queryFn: () => getSetting(key),
  });
}
