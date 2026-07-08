import { useMemo } from "react";
import { ExportActions } from "@/components/export/ExportActions";
import { CategoryTotalsCard } from "@/components/stats/CategoryTotalsCard";
import { DailyBreakdownCard } from "@/components/stats/DailyBreakdownCard";
import { NeedsAttentionCard } from "@/components/stats/NeedsAttentionCard";
import { PeriodProgressCard } from "@/components/stats/PeriodProgressCard";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import type { Category, Event, Period, ReviewItem } from "@/lib/api";
import { buildPeriodExportSummary } from "@/lib/export";
import { errorMessage, localDateKey, type ScheduleGapOverlay } from "@/lib/schedule";
import type { ScheduleItem } from "@/lib/schedule/types";

interface ScheduleSidebarProps {
  activePeriod: Period | null;
  categories: Category[];
  items: ScheduleItem[];
  events: Event[];
  reviewItems: ReviewItem[];
  visibleGaps: ScheduleGapOverlay[];
  onOpenReviewQueue?: () => void;
  isBackendLoading: boolean;
  backendError: unknown;
}

export function ScheduleSidebar({
  activePeriod,
  categories,
  items,
  events,
  reviewItems,
  visibleGaps,
  onOpenReviewQueue,
  isBackendLoading,
  backendError,
}: ScheduleSidebarProps) {
  const today = useMemo(() => localDateKey(), []);
  const exportSummary = useMemo(() => {
    if (!activePeriod) {
      return null;
    }

    return buildPeriodExportSummary(items, activePeriod, { categories });
  }, [activePeriod, categories, items]);

  return (
    <ScrollArea className="app-no-drag h-full min-h-0">
      <div className="flex min-h-full flex-col gap-3 p-1 pr-3">
        <PeriodProgressCard summary={exportSummary} today={today} />
        <CategoryTotalsCard summary={exportSummary} />
        <NeedsAttentionCard
          reviewItems={reviewItems}
          events={events}
          visibleGaps={visibleGaps}
          onOpenReviewQueue={onOpenReviewQueue}
        />
        <DailyBreakdownCard summary={exportSummary} />
        <div className="mt-auto space-y-2 pt-1">
          {isBackendLoading ? (
            <p className="rounded-md border border-border bg-background p-2 text-xs text-muted-foreground">
              Loading backend data
            </p>
          ) : null}
          {backendError ? (
            <p className="rounded-md border border-destructive/30 bg-destructive/10 p-2 text-xs text-destructive">
              {errorMessage(backendError)}
            </p>
          ) : null}
          <ExportActions
            summary={exportSummary}
            disabled={!activePeriod || isBackendLoading}
            layout="stacked"
          />
          <Button
            type="button"
            className="w-full"
            disabled
            title="Finalize period is not available yet"
          >
            Finalize period
          </Button>
        </div>
      </div>
    </ScrollArea>
  );
}
