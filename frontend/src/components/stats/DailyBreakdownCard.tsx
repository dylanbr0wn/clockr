import { ChevronDownIcon } from "lucide-react";
import { useState } from "react";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import {
  sortedCategoryNames,
  varianceMinutes,
  type PeriodExportSummary,
} from "@/lib/export";
import { formatVariance } from "@/lib/export/formatters";
import { formatDateKey, formatDuration } from "@/lib/schedule";
import { categoryStatColor } from "@/lib/category/colors";
import { cn } from "@/lib/utils";
import { varianceToneClass } from "./statsShared";

interface DailyBreakdownCardProps {
  summary: PeriodExportSummary | null;
  className?: string;
}

export function DailyBreakdownCard({
  summary,
  className,
}: DailyBreakdownCardProps) {
  const [open, setOpen] = useState(false);

  if (!summary || summary.dailyTotals.length === 0) {
    return null;
  }

  return (
    <Card className={cn("app-no-drag", className)}>
      <CardHeader className="flex items-center justify-between gap-2 space-y-0">
        <CardTitle className="text-sm">Daily breakdown</CardTitle>
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          className="shrink-0"
          aria-expanded={open}
          onClick={() => setOpen((current) => !current)}
        >
          <ChevronDownIcon
            className={cn("size-4 transition-transform", open && "rotate-180")}
          />
        </Button>
      </CardHeader>
      {open ? (
        <CardContent className="space-y-3">
          {summary.dailyTotals.map((day) => {
            const dayCategories = sortedCategoryNames(day.categories);
            const dayVariance = varianceMinutes(
              day.actualMinutes,
              day.targetMinutes,
            );

            return (
              <div
                key={day.date}
                className="rounded-md border border-border bg-muted/30 p-3"
              >
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <p className="text-sm font-medium text-foreground">
                      {formatDateKey(day.date)}
                    </p>
                    <p className="text-xs text-muted-foreground">{day.date}</p>
                  </div>
                  <div className="text-right text-sm">
                    <p className="font-semibold text-foreground">
                      {formatDuration(day.actualMinutes)}
                      <span className="font-medium text-muted-foreground">
                        {" "}
                        / {formatDuration(day.targetMinutes)}
                      </span>
                    </p>
                    <p
                      className={cn(
                        "text-xs font-medium",
                        varianceToneClass(dayVariance),
                      )}
                    >
                      {formatVariance(day.actualMinutes, day.targetMinutes)}
                    </p>
                  </div>
                </div>
                {dayCategories.length > 0 ? (
                  <div className="mt-2 space-y-1 border-t border-border/70 pt-2">
                    {dayCategories.map((category) => (
                      <div
                        key={category}
                        className="flex items-center justify-between gap-3 text-xs"
                      >
                        <span className="flex min-w-0 items-center gap-2 text-muted-foreground">
                          <span
                            className="size-2 shrink-0 rounded-sm"
                            style={{
                              backgroundColor: categoryStatColor(
                                category,
                                summary.categoryColors,
                              ),
                            }}
                            aria-hidden
                          />
                          <span className="truncate">{category}</span>
                        </span>
                        <span className="font-medium text-foreground">
                          {formatDuration(day.categories[category])}
                        </span>
                      </div>
                    ))}
                  </div>
                ) : (
                  <p className="mt-2 border-t border-border/70 pt-2 text-xs text-muted-foreground">
                    No tracked time
                  </p>
                )}
              </div>
            );
          })}
        </CardContent>
      ) : null}
    </Card>
  );
}
