import type { Event, ReviewDecision } from "@/lib/api";
import { formatDecimalHours } from "@/lib/export/formatters";
import type { ScheduleGapOverlay } from "@/lib/schedule";

export type AttentionIconTone = "amber" | "sky" | "rose";

export interface AttentionItem {
  id: string;
  iconTone: AttentionIconTone;
  title: string;
  subtitle: string;
  actionLabel: string;
}

function gapDurationMinutes(gap: ScheduleGapOverlay) {
  return Math.max(0, gap.endMinutes - gap.startMinutes);
}

export function buildAttentionItems({
  reviewDecisions,
  visibleGaps,
}: {
  reviewDecisions: ReviewDecision[];
  events: Event[];
  visibleGaps: ScheduleGapOverlay[];
}): AttentionItem[] {
  const items: AttentionItem[] = [];

  if (visibleGaps.length > 0) {
    const totalGapMinutes = visibleGaps.reduce(
      (total, gap) => total + gapDurationMinutes(gap),
      0,
    );

    items.push({
      id: "gaps",
      iconTone: "sky",
      title:
        visibleGaps.length === 1
          ? "1 unfilled gap"
          : `${visibleGaps.length} unfilled gaps`,
      subtitle: `${formatDecimalHours(totalGapMinutes)} uncategorized`,
      actionLabel: "Fill",
    });
  }

  const deleted = reviewDecisions.filter(
    (decision) => decision.kind === "deleted_categorized",
  );
  if (deleted.length > 0) {
    items.push({
      id: "deleted",
      iconTone: "rose",
      title:
        deleted.length === 1
          ? "1 deleted event"
          : `${deleted.length} deleted events`,
      subtitle: "Was categorized · review removal",
      actionLabel: "Review",
    });
  }

  const otherReview = reviewDecisions.filter(
    (decision) => decision.kind !== "deleted_categorized",
  );

  if (otherReview.length > 0) {
    items.push({
      id: "review",
      iconTone: "amber",
      title:
        otherReview.length === 1
          ? "1 item needs review"
          : `${otherReview.length} items need review`,
      subtitle: "Open the review queue to resolve",
      actionLabel: "Review",
    });
  }

  return items;
}
