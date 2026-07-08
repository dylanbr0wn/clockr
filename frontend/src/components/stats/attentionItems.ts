import type { Event, ReviewItem } from "@/lib/api";
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

interface ReviewPayload {
  title?: string;
  from?: string;
  to?: string;
}

function parsePayload(payload: string): ReviewPayload {
  try {
    return JSON.parse(payload) as ReviewPayload;
  } catch {
    return {};
  }
}

function eventTitle(event: Event | undefined, payload: ReviewPayload) {
  return event?.title || payload.title || "Untitled event";
}

function gapDurationMinutes(gap: ScheduleGapOverlay) {
  return Math.max(0, gap.endMinutes - gap.startMinutes);
}

function overlapSubtitle(items: ReviewItem[], eventsById: Map<number, Event>) {
  const first = items[0];
  const payload = parsePayload(first.payload);
  const event =
    typeof first.eventId === "number" ? eventsById.get(first.eventId) : undefined;
  const title = eventTitle(event, payload);

  if (items.length === 1) {
    return title;
  }

  return `${title} and ${items.length - 1} more`;
}

export function buildAttentionItems({
  reviewItems,
  events,
  visibleGaps,
}: {
  reviewItems: ReviewItem[];
  events: Event[];
  visibleGaps: ScheduleGapOverlay[];
}): AttentionItem[] {
  const eventsById = new Map(events.map((event) => [event.id, event]));
  const openItems = reviewItems.filter((item) => item.status === "open");
  const items: AttentionItem[] = [];

  const overlaps = openItems.filter((item) => item.kind === "overlap");
  if (overlaps.length > 0) {
    items.push({
      id: "overlap",
      iconTone: "amber",
      title:
        overlaps.length === 1
          ? "1 overlapping event"
          : `${overlaps.length} overlapping events`,
      subtitle: overlapSubtitle(overlaps, eventsById),
      actionLabel: "Resolve",
    });
  }

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

  const deleted = openItems.filter((item) => item.kind === "deleted_categorized");
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

  const otherReview = openItems.filter(
    (item) =>
      item.kind !== "overlap" &&
      item.kind !== "deleted_categorized" &&
      item.kind !== "dedup_ambiguous",
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
