import type { GapFill } from "@/lib/api";

export function eventItemId(eventId: number): string {
  return `event-${eventId}`;
}

export function gapFillItemId(gapFillId: number): string {
  return `gap-fill-${gapFillId}`;
}

export function allDayChipId(eventId: number, day: string): string {
  return `${eventItemId(eventId)}@${day}`;
}

export function parseEventItemId(itemId: string): number | null {
  const match = /^event-(\d+)$/.exec(itemId);
  if (!match) {
    return null;
  }

  const eventId = Number(match[1]);
  return Number.isFinite(eventId) ? eventId : null;
}

export function parseGapFillItemId(itemId: string): number | null {
  const match = /^gap-fill-(\d+)$/.exec(itemId);
  if (!match) {
    return null;
  }

  const gapFillId = Number(match[1]);
  return Number.isFinite(gapFillId) ? gapFillId : null;
}

export function isEventItemId(itemId: string): boolean {
  return parseEventItemId(itemId) !== null;
}

export function isGapFillItemId(itemId: string): boolean {
  return parseGapFillItemId(itemId) !== null;
}

export function buildGapFillsByItemId(
  gapFills: GapFill[],
): Map<string, GapFill> {
  return new Map(gapFills.map((gapFill) => [gapFillItemId(gapFill.id), gapFill]));
}
