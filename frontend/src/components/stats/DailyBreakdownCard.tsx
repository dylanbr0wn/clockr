import { ChevronDownIcon } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Item,
  ItemContent,
  ItemDescription,
  ItemGroup,
  ItemTitle,
} from "@/components/ui/item";
import {
  sortedCategoryNames,
  varianceMinutes,
  type PeriodExportSummary,
} from "@/lib/export";
import { formatVariance } from "@/lib/export/formatters";
import { categoryStatColor } from "@/lib/category/colors";
import { formatDateKey, formatDuration } from "@/lib/schedule";
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
        <CardContent>
          <ItemGroup className="gap-3">
            {summary.dailyTotals.map((day) => {
              const dayCategories = sortedCategoryNames(day.categories);
              const dayVariance = varianceMinutes(
                day.actualMinutes,
                day.targetMinutes,
              );

              return (
                <Item key={day.date} variant="muted" size="sm">
                  <ItemContent className="w-full">
                    <div className="flex items-start justify-between gap-3">
                      <div>
                        <ItemTitle>{formatDateKey(day.date)}</ItemTitle>
                        <ItemDescription>{day.date}</ItemDescription>
                      </div>
                      <div className="text-right">
                        <ItemTitle>
                          {formatDuration(day.actualMinutes)}
                          <span className="font-medium text-muted-foreground">
                            {" "}
                            / {formatDuration(day.targetMinutes)}
                          </span>
                        </ItemTitle>
                        <ItemDescription
                          className={cn(
                            "font-medium",
                            varianceToneClass(dayVariance),
                          )}
                        >
                          {formatVariance(day.actualMinutes, day.targetMinutes)}
                        </ItemDescription>
                      </div>
                    </div>
                    {dayCategories.length > 0 ? (
                      <ItemGroup className="mt-2 gap-1 border-t border-border/70 pt-2">
                        {dayCategories.map((category) => (
                          <Item key={category} size="xs">
                            <ItemContent className="min-w-0">
                              <ItemDescription className="flex items-center gap-2 text-xs">
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
                              </ItemDescription>
                            </ItemContent>
                            <ItemContent className="flex-none">
                              <ItemTitle className="text-xs">
                                {formatDuration(day.categories[category])}
                              </ItemTitle>
                            </ItemContent>
                          </Item>
                        ))}
                      </ItemGroup>
                    ) : (
                      <ItemDescription className="mt-2 border-t border-border/70 pt-2 text-xs">
                        No tracked time
                      </ItemDescription>
                    )}
                  </ItemContent>
                </Item>
              );
            })}
          </ItemGroup>
        </CardContent>
      ) : null}
    </Card>
  );
}
