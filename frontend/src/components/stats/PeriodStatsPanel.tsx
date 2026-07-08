import type { PeriodExportSummary } from "@/lib/export";
import { CategoryTotalsCard } from "./CategoryTotalsCard";
import { DailyBreakdownCard } from "./DailyBreakdownCard";
import { PeriodProgressCard } from "./PeriodProgressCard";

interface PeriodStatsPanelProps {
  summary: PeriodExportSummary | null;
  today: string;
  className?: string;
}

export function PeriodStatsPanel({
  summary,
  today,
  className,
}: PeriodStatsPanelProps) {
  return (
    <>
      <PeriodProgressCard summary={summary} today={today} className={className} />
      <CategoryTotalsCard summary={summary} className={className} />
      <DailyBreakdownCard summary={summary} className={className} />
    </>
  );
}
