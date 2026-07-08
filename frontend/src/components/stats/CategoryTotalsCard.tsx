import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { sortCategoriesByMinutes, type PeriodExportSummary } from "@/lib/export";
import { formatDecimalHours } from "@/lib/export/formatters";
import { categoryColor } from "@/lib/schedule/categoryColors";
import { cn } from "@/lib/utils";
import { categoryBarWidth } from "./statsShared";

interface CategoryTotalsCardProps {
  summary: PeriodExportSummary | null;
  className?: string;
}

export function CategoryTotalsCard({
  summary,
  className,
}: CategoryTotalsCardProps) {
  if (!summary) {
    return null;
  }

  const categories = sortCategoriesByMinutes(summary.periodTotals);
  const maxCategoryMinutes = Math.max(
    ...categories.map((category) => summary.periodTotals[category]),
    1,
  );

  return (
    <Card className={cn("app-no-drag", className)}>
      <CardHeader>
        <CardTitle className="text-sm">Totals by category</CardTitle>
      </CardHeader>
      <CardContent>
        {categories.length > 0 ? (
          <div className="space-y-2">
            {categories.map((category) => {
              const minutes = summary.periodTotals[category];
              const colors = categoryColor(category);

              return (
                <div key={category} className="flex items-center gap-2 text-[13px]">
                  <span
                    className={cn("size-2 shrink-0 rounded-sm", colors.dot)}
                    aria-hidden
                  />
                  <span className="min-w-0 flex-1 truncate text-foreground">
                    {category}
                  </span>
                  <span className="h-1 w-16 shrink-0 overflow-hidden rounded-full bg-muted">
                    <span
                      className={cn("block h-full rounded-full", colors.bar)}
                      style={{
                        width: `${categoryBarWidth(minutes, maxCategoryMinutes)}%`,
                      }}
                    />
                  </span>
                  <span className="shrink-0 font-semibold tabular-nums text-foreground">
                    {formatDecimalHours(minutes)}
                  </span>
                </div>
              );
            })}
          </div>
        ) : (
          <p className="text-sm text-muted-foreground">No tracked time yet.</p>
        )}
      </CardContent>
    </Card>
  );
}
