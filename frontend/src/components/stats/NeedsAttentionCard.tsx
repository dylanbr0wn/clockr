import {
  AlertTriangleIcon,
  CalendarIcon,
  Trash2Icon,
} from "lucide-react";
import { useMemo } from "react";
import type { Event, ReviewDecision } from "@/lib/api";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Item,
  ItemActions,
  ItemContent,
  ItemDescription,
  ItemGroup,
  ItemMedia,
  ItemTitle,
} from "@/components/ui/item";
import type { ScheduleGapOverlay } from "@/lib/schedule";
import { cn } from "@/lib/utils";
import {
  buildAttentionItems,
  type AttentionIconTone,
  type AttentionItem,
} from "./attentionItems";

interface NeedsAttentionCardProps {
  reviewDecisions: ReviewDecision[];
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
    <Item variant="muted" size="sm">
      <ItemMedia>
        <AttentionIcon tone={item.iconTone} />
      </ItemMedia>
      <ItemContent>
        <ItemTitle className="text-[12.5px] leading-tight">{item.title}</ItemTitle>
        <ItemDescription className="text-[11.5px]">{item.subtitle}</ItemDescription>
      </ItemContent>
      {onAction ? (
        <ItemActions>
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="h-[26px] shrink-0 px-2.5 text-xs"
            onClick={onAction}
          >
            {item.actionLabel}
          </Button>
        </ItemActions>
      ) : null}
    </Item>
  );
}

export function NeedsAttentionCard({
  reviewDecisions,
  events,
  visibleGaps,
  onOpenReviewQueue,
  className,
}: NeedsAttentionCardProps) {
  const items = useMemo(
    () => buildAttentionItems({ reviewDecisions, events, visibleGaps }),
    [events, reviewDecisions, visibleGaps],
  );

  if (items.length === 0) {
    return null;
  }

  return (
    <Card className={cn("app-no-drag", className)}>
      <CardHeader>
        <CardTitle className="text-sm">Needs attention</CardTitle>
      </CardHeader>
      <CardContent>
        <ItemGroup className="gap-2">
          {items.map((item) => (
            <AttentionRow
              key={item.id}
              item={item}
              onAction={onOpenReviewQueue}
            />
          ))}
        </ItemGroup>
      </CardContent>
    </Card>
  );
}
