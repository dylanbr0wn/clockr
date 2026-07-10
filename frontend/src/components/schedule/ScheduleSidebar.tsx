import { useMemo } from "react";
import { ExportActions } from "@/components/export/ExportActions";
import { CategoryTotalsCard } from "@/components/stats/CategoryTotalsCard";
import { DailyBreakdownCard } from "@/components/stats/DailyBreakdownCard";
import { NeedsAttentionCard } from "@/components/stats/NeedsAttentionCard";
import { PeriodProgressCard } from "@/components/stats/PeriodProgressCard";
import { Button } from "@/components/ui/button";
import {
  Item,
  ItemContent,
  ItemDescription,
} from "@/components/ui/item";
import { ScrollArea } from "@/components/ui/scroll-area";
import type { Category, Event, Period, ReviewDecision } from "@/lib/api";
import { buildPeriodExportSummary } from "@/lib/export";
import { errorMessage, localDateKey, type ScheduleGapOverlay } from "@/lib/schedule";
import type { ScheduleItem } from "@/lib/schedule/types";

interface ScheduleSidebarProps {
  activePeriod: Period | null;
  categories: Category[];
  items: ScheduleItem[];
  events: Event[];
  reviewDecisions: ReviewDecision[];
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
  reviewDecisions,
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
          reviewDecisions={reviewDecisions}
          events={events}
          visibleGaps={visibleGaps}
          onOpenReviewQueue={onOpenReviewQueue}
        />
        <DailyBreakdownCard summary={exportSummary} />
        <div className="mt-auto flex flex-col gap-2 pt-1">
          {isBackendLoading ? (
            <Item variant="outline" size="xs">
              <ItemContent>
                <ItemDescription>Loading backend data</ItemDescription>
              </ItemContent>
            </Item>
          ) : null}
          {backendError ? (
            <Item
              variant="outline"
              size="xs"
              className="border-destructive/30 bg-destructive/10"
            >
              <ItemContent>
                <ItemDescription className="text-destructive">
                  {errorMessage(backendError)}
                </ItemDescription>
              </ItemContent>
            </Item>
          ) : null}
          <ExportActions
            summary={exportSummary}
            items={items}
            periodId={activePeriod?.id}
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
