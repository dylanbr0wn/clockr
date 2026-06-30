import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { formatMinutes } from "@/lib/scheduler";
import {
  errorMessage,
  formatCadence,
  formatDuration,
  type ScheduleChange,
} from "@/lib/schedule";
import type { Period } from "@/lib/api";

interface ScheduleSidebarProps {
  activePeriod: Period | null;
  visibleDayCount: number;
  totals: Record<string, number>;
  preview: ScheduleChange | null;
  counts: {
    events: number;
    gapFills: number;
    categories: number;
    reviewItems: number;
  };
  isBackendLoading: boolean;
  backendError: unknown;
}

export function ScheduleSidebar({
  activePeriod,
  visibleDayCount,
  totals,
  preview,
  counts,
  isBackendLoading,
  backendError,
}: ScheduleSidebarProps) {
  return (
    <div className="flex flex-col gap-4">
      <Card className="app-no-drag min-h-0 space-y-4 overflow-auto overscroll-none">
        <CardHeader>
          <CardTitle className="text-sm">Totals by category</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="mt-3 space-y-2">
            {Object.entries(totals).map(([category, minutes]) => (
              <div
                key={category}
                className="flex items-center justify-between gap-3 text-sm"
              >
                <span className="truncate text-muted-foreground">{category}</span>
                <span className="font-semibold text-foreground">
                  {formatDuration(minutes)}
                </span>
              </div>
            ))}
          </div>
          <div className="border-t border-border pt-4">
            <h2 className="text-sm font-semibold text-foreground">Preview</h2>
            <div className="mt-3 min-h-16 rounded-md border border-border bg-muted p-3 text-sm text-muted-foreground">
              {preview ? (
                <div className="space-y-1">
                  <p className="font-medium text-foreground">
                    {preview.interaction}
                  </p>
                  <p>{preview.day}</p>
                  <p>
                    {formatMinutes(preview.startMinutes)}-
                    {formatMinutes(preview.endMinutes)}
                  </p>
                </div>
              ) : (
                <p>Idle</p>
              )}
            </div>
          </div>
        </CardContent>
      </Card>
      <Card className="app-no-drag">
        <CardHeader>
          <CardTitle className="text-sm">Schedule</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-3 text-sm">
            <div className="flex items-center justify-between gap-3">
              <span className="text-muted-foreground">Period</span>
              <span className="truncate font-medium">
                {activePeriod
                  ? `${activePeriod.startDate} to ${activePeriod.endDate}`
                  : "No period"}
              </span>
            </div>
            {activePeriod && (
              <>
                <div className="flex items-center justify-between gap-3">
                  <span className="text-muted-foreground">Cadence</span>
                  <span className="font-medium">
                    {formatCadence(activePeriod.cadence)}
                  </span>
                </div>
                <div className="flex items-center justify-between gap-3">
                  <span className="text-muted-foreground">Days</span>
                  <span className="font-medium">{visibleDayCount}</span>
                </div>
                <div className="flex items-center justify-between gap-3">
                  <span className="text-muted-foreground">Target</span>
                  <span className="font-medium">
                    {activePeriod.targetHoursPerDay}h/day
                  </span>
                </div>
              </>
            )}
            <div className="flex items-center justify-between gap-3">
              <span className="text-muted-foreground">Events</span>
              <span className="font-medium">{counts.events}</span>
            </div>
            <div className="flex items-center justify-between gap-3">
              <span className="text-muted-foreground">Gap fills</span>
              <span className="font-medium">{counts.gapFills}</span>
            </div>
            <div className="flex items-center justify-between gap-3">
              <span className="text-muted-foreground">Categories</span>
              <span className="font-medium">{counts.categories}</span>
            </div>
            <div className="flex items-center justify-between gap-3">
              <span className="text-muted-foreground">Review</span>
              <span className="font-medium">{counts.reviewItems}</span>
            </div>
            {isBackendLoading && (
              <p className="rounded-md border border-border bg-background p-2 text-xs text-muted-foreground">
                Loading backend data
              </p>
            )}
            {backendError ? (
              <p className="rounded-md border border-destructive/30 bg-destructive/10 p-2 text-xs text-destructive">
                {errorMessage(backendError)}
              </p>
            ) : null}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
