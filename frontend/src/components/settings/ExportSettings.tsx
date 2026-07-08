import { useMemo } from "react";
import { useCurrentPeriod } from "@/lib/api";
import { localDateKey } from "@/lib/schedule";
import { defaultTimeZone } from "@/lib/schedule/timezone";
import { ExportActions } from "@/components/export/ExportActions";
import { PeriodStatsPanel } from "@/components/stats/PeriodStatsPanel";
import { SettingBlock } from "./SettingBlock";
import { usePeriodExport } from "@/lib/export";

export function ExportSettings() {
  const today = useMemo(() => localDateKey(), []);
  const timeZone = useMemo(() => defaultTimeZone(), []);
  const currentPeriodQuery = useCurrentPeriod(today, timeZone);
  const period = currentPeriodQuery.data ?? null;
  const exportData = usePeriodExport(period);

  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <SettingBlock
        title="Period export"
        description="Review period stats, then copy a text summary or download a CSV for the current pay period."
      >
        <div className="flex flex-col gap-4">
          {exportData.isLoading || currentPeriodQuery.isLoading ? (
            <p className="text-sm text-muted-foreground">Loading period totals</p>
          ) : (
            <PeriodStatsPanel summary={exportData.summary} today={today} />
          )}
          <ExportActions
            summary={exportData.summary}
            disabled={exportData.isLoading || currentPeriodQuery.isLoading}
            layout="stacked"
          />
        </div>
      </SettingBlock>
    </div>
  );
}
