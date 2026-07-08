import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  periodProgressPercent,
  varianceMinutes,
  type PeriodExportSummary,
} from "@/lib/export";
import { formatDecimalHours, formatVariance } from "@/lib/export/formatters";
import { workingDaysRemaining } from "@/lib/schedule/date";
import { cn } from "@/lib/utils";
import { varianceToneClass } from "./statsShared";

interface PeriodProgressCardProps {
  summary: PeriodExportSummary | null;
  today: string;
  className?: string;
}

export function PeriodProgressCard({
  summary,
  today,
  className,
}: PeriodProgressCardProps) {
  if (!summary) {
    return (
      <Card className={cn("app-no-drag", className)}>
        <CardHeader>
          <CardTitle className="text-sm">Period progress</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">No active period.</p>
        </CardContent>
      </Card>
    );
  }

  const progressPercent = periodProgressPercent(summary);
  const progressWidth = Math.min(progressPercent, 100);
  const variance = varianceMinutes(summary.actualMinutes, summary.targetMinutes);
  const daysLeft = workingDaysRemaining(summary, today);

  return (
    <Card className={cn("app-no-drag", className)}>
      <CardHeader className="gap-0.5">
        <CardTitle className="text-sm">Period progress</CardTitle>
        <p className="text-xs text-muted-foreground">
          {daysLeft === 1 ? "1 working day left" : `${daysLeft} working days left`}
        </p>
      </CardHeader>
      <CardContent className="space-y-2">
        <div className="flex items-end justify-between gap-3">
          <p className="text-[22px] font-semibold leading-none tracking-tight text-foreground">
            {formatDecimalHours(summary.actualMinutes)}
            <span className="text-[13px] font-normal text-muted-foreground">
              {" "}
              / {formatDecimalHours(summary.targetMinutes)}
            </span>
          </p>
          <span className="text-sm font-semibold text-foreground">
            {progressPercent}%
          </span>
        </div>
        <div className="h-2 overflow-hidden rounded-full bg-muted">
          <div
            className="h-full rounded-full bg-primary transition-[width]"
            style={{ width: `${progressWidth}%` }}
          />
        </div>
        <p className={cn("text-xs font-medium", varianceToneClass(variance))}>
          {formatVariance(summary.actualMinutes, summary.targetMinutes)} vs target
          {" · "}
          {summary.targetHoursPerDay}h/day
        </p>
      </CardContent>
    </Card>
  );
}
