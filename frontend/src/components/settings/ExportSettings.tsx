import { useMemo } from "react";
import { useCurrentPeriod } from "@/lib/api";
import { formatDuration, localDateKey } from "@/lib/schedule";
import { defaultTimeZone } from "@/lib/schedule/timezone";
import { ExportActions } from "@/components/export/ExportActions";
import { SettingBlock } from "./SettingBlock";
import { sortedCategories, usePeriodExport } from "@/lib/export";

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
        description="Copy a text summary or download a CSV for the current pay period."
      >
        <div className="space-y-4">
          <div className="rounded-md border border-border bg-muted/40 p-4 text-sm">
            <p className="font-medium text-foreground">
              {period
                ? `${period.startDate} to ${period.endDate}`
                : "No active period"}
            </p>
            {exportData.summary ? (
              <div className="mt-3 space-y-2">
                {sortedCategories(exportData.summary).map((category) => (
                  <div
                    key={category}
                    className="flex items-center justify-between gap-3"
                  >
                    <span className="text-muted-foreground">{category}</span>
                    <span className="font-medium">
                      {formatDuration(exportData.summary!.periodTotals[category])}
                    </span>
                  </div>
                ))}
              </div>
            ) : (
              <p className="mt-2 text-muted-foreground">
                {exportData.isLoading
                  ? "Loading period totals"
                  : "No tracked time yet for this period."}
              </p>
            )}
          </div>
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
