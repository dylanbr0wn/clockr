import {
  AlertTriangleIcon,
  CalendarIcon,
  Trash2Icon,
} from "lucide-react";
import { useMemo } from "react";
import type { Event, ReviewItem } from "@/lib/api";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import type { ScheduleGapOverlay } from "@/lib/schedule";
import { cn } from "@/lib/utils";
import {
  buildAttentionItems,
  type AttentionIconTone,
  type AttentionItem,
} from "./attentionItems";

interface NeedsAttentionCardProps {
  reviewItems: ReviewItem[];
  events: Event[];
  visibleGaps: ScheduleGapOverlay[];
  onOpenReviewQueue?: () => void;
  className?: string;
}

const iconToneClasses: Record<AttentionIconTone, string> = {
  amber: "bg-amber-50 text-amber-800",
  sky: "bg-sky-50 text-sky-800",
  rose: "bg-rose-50 text-rose-800",
};

function AttentionIcon({ tone }: { tone: AttentionIconTone }) {
  const className = cn(
    "flex size-[26px] shrink-0 items-center justify-center rounded-md",
    iconToneClasses[tone],
  );

  switch (tone) {
    case "amber":
      return (
        <span className={className}>
          <AlertTriangleIcon className="size-3.5" />
        </span>
      );
    case "sky":
      return (
        <span className={className}>
          <CalendarIcon className="size-3.5" />
        </span>
      );
    case "rose":
      return (
        <span className={className}>
          <Trash2Icon className="size-3.5" />
        </span>
      );
  }
}

function AttentionRow({
  item,
  onAction,
}: {
  item: AttentionItem;
  onAction?: () => void;
}) {
  return (
    <div className="flex items-center gap-2.5 rounded-md border border-border bg-muted px-2.5 py-2">
      <AttentionIcon tone={item.iconTone} />
      <div className="min-w-0 flex-1">
        <p className="text-[12.5px] font-medium leading-tight text-foreground">
          {item.title}
        </p>
        <p className="text-[11.5px] text-muted-foreground">{item.subtitle}</p>
      </div>
      {onAction ? (
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="h-[26px] shrink-0 px-2.5 text-xs"
          onClick={onAction}
        >
          {item.actionLabel}
        </Button>
      ) : null}
    </div>
  );
}

export function NeedsAttentionCard({
  reviewItems,
  events,
  visibleGaps,
  onOpenReviewQueue,
  className,
}: NeedsAttentionCardProps) {
  const items = useMemo(
    () => buildAttentionItems({ reviewItems, events, visibleGaps }),
    [events, reviewItems, visibleGaps],
  );

  if (items.length === 0) {
    return null;
  }

  return (
    <Card className={cn("app-no-drag", className)}>
      <CardHeader>
        <CardTitle className="text-sm">Needs attention</CardTitle>
      </CardHeader>
      <CardContent className="space-y-2">
        {items.map((item) => (
          <AttentionRow
            key={item.id}
            item={item}
            onAction={onOpenReviewQueue}
          />
        ))}
      </CardContent>
    </Card>
  );
}
