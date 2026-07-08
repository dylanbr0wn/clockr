import { anyLoading, firstError } from "./useSchedulePage.helpers";

interface BuildSchedulePageStatusArgs {
  loadingFlags: boolean[];
  errors: unknown[];
  eventsCount: number;
  gapFillsCount: number;
  categoriesCount: number;
  reviewItemsCount: number;
}

export function buildSchedulePageStatus({
  loadingFlags,
  errors,
  eventsCount,
  gapFillsCount,
  categoriesCount,
  reviewItemsCount,
}: BuildSchedulePageStatusArgs) {
  return {
    isBackendLoading: anyLoading(loadingFlags),
    backendError: firstError(errors),
    counts: {
      events: eventsCount,
      gapFills: gapFillsCount,
      categories: categoriesCount,
      reviewItems: reviewItemsCount,
    },
  };
}
