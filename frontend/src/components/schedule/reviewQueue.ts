import type { Event, ReviewItem } from "@/lib/api";
import { formatDuration } from "@/lib/schedule";

export type ReviewItemKind =
  | "new_in_gap"
  | "title_changed"
  | "deleted_categorized"
  | "dedup_ambiguous"
  | "overlap"
  | "tentative"
  | "all_day";

export interface ReviewPayload {
  title?: string;
  from?: string;
  to?: string;
  reason?: string;
  status?: string;
}

export interface ReviewItemView {
  id: number;
  kind: ReviewItemKind;
  tag: string;
  title: string;
  description: string;
  primaryAction: { label: string; action: string };
  secondaryAction?: { label: string; action: string };
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

function eventMinutes(event: Event | undefined) {
  if (!event?.start || !event?.end) {
    return null;
  }

  const start = new Date(event.start).getTime();
  const end = new Date(event.end).getTime();
  if (!Number.isFinite(start) || !Number.isFinite(end) || end <= start) {
    return null;
  }

  return Math.round((end - start) / 60_000);
}

export function buildReviewItemView(
  item: ReviewItem,
  eventsById: Map<number, Event>,
): ReviewItemView | null {
  const payload = parsePayload(item.payload);
  const event =
    typeof item.eventId === "number"
      ? eventsById.get(item.eventId)
      : undefined;
  const title = eventTitle(event, payload);
  const minutes = eventMinutes(event);
  const kind = item.kind as ReviewItemKind;

  switch (kind) {
    case "deleted_categorized": {
      const reason =
        payload.reason === "declined" ? "declined on your calendar" : "deleted from your calendar";
      return {
        id: item.id,
        kind,
        tag: "Removed",
        title,
        description: `${reason} but already categorized${minutes ? ` (${formatDuration(minutes)})` : ""}. Suggest dropping the entry.`,
        primaryAction: { label: minutes ? `Drop ${formatDuration(minutes)}` : "Drop entry", action: "drop_entry" },
        secondaryAction: { label: "Keep entry", action: "keep_entry" },
      };
    }
    case "title_changed":
      return {
        id: item.id,
        kind,
        tag: "Title changed",
        title,
        description: `Title changed from "${payload.from ?? "previous"}" to "${payload.to ?? title}". Confirm the existing category still applies.`,
        primaryAction: { label: "Accept new title", action: "accept" },
        secondaryAction: { label: "Remind me later", action: "dismiss" },
      };
    case "new_in_gap":
      return {
        id: item.id,
        kind,
        tag: "Gap conflict",
        title,
        description: `"${title}" landed inside a gap you already filled. Keeping both would double-count time.`,
        primaryAction: { label: "Use event, shrink fill", action: "use_event" },
        secondaryAction: { label: "Keep gap fill", action: "keep_gap" },
      };
    case "tentative":
      return {
        id: item.id,
        kind,
        tag: "Tentative",
        title,
        description: `Marked ${payload.status === "needsAction" ? "not responded" : "tentative"}. Include it in your schedule or exclude it.`,
        primaryAction: { label: "Include", action: "include" },
        secondaryAction: { label: "Exclude", action: "exclude" },
      };
    case "all_day":
      return {
        id: item.id,
        kind,
        tag: "All day",
        title,
        description:
          "All-day markers do not affect gap totals. Include to confirm, or dismiss the prompt.",
        primaryAction: { label: "Include", action: "include" },
        secondaryAction: { label: "Dismiss", action: "exclude" },
      };
    case "overlap":
    case "dedup_ambiguous":
      return null;
    default:
      return null;
  }
}

export function buildReviewItemViews(
  items: ReviewItem[],
  events: Event[],
): ReviewItemView[] {
  const eventsById = new Map(events.map((event) => [event.id, event]));

  return items
    .map((item) => buildReviewItemView(item, eventsById))
    .filter((item): item is ReviewItemView => item !== null);
}
