import type { TimeEntry } from "@/lib/api";
import { toDate, zonedDateTimeParts } from "./timezone";

/** Keep one confirmed entry bucketed on the start date. */
export const OVERNIGHT_ATTRIBUTE_TO_START = "attribute_to_start";
/** Confirm midnight-cut segments and dismiss the original draft. */
export const OVERNIGHT_SPLIT_AT_MIDNIGHT = "split_at_midnight";

/** Confirm despite overlapping confirmed entries (one-shot). */
export const OVERLAP_ALLOW_PARALLEL = "allow_parallel";
/** Dismiss the draft and leave overlapping confirmed entries. */
export const OVERLAP_KEEP_THEIRS = "keep_theirs";
/** Delete overlapping confirmed entries, then confirm the draft. */
export const OVERLAP_KEEP_MINE = "keep_mine";
/** Confirm only the non-overlapping remainder(s) of the draft. */
export const OVERLAP_SPLIT = "split";

/** True when local calendar day of start ≠ local day of end. */
export function entryCrossesLocalMidnight(
  startIso: string,
  endIso: string,
  timeZone: string,
): boolean {
  const start = toDate(startIso);
  const end = toDate(endIso);
  if (!start || !end) {
    return false;
  }

  return (
    zonedDateTimeParts(start, timeZone).day !==
    zonedDateTimeParts(end, timeZone).day
  );
}

/** Interval overlap with other confirmed entries (exclude self by id). */
export function findOverlappingConfirmed(
  draft: { id: number; start: string; end: string },
  confirmed: TimeEntry[],
): TimeEntry[] {
  const start = toDate(draft.start);
  const end = toDate(draft.end);
  if (!start || !end) {
    return [];
  }

  return confirmed.filter((entry) => {
    if (entry.id === draft.id || entry.attestation !== "confirmed") {
      return false;
    }
    const otherStart = toDate(entry.start);
    const otherEnd = toDate(entry.end);
    if (!otherStart || !otherEnd) {
      return false;
    }
    return otherStart.getTime() < end.getTime() && otherEnd.getTime() > start.getTime();
  });
}

/** Sorted ascending cut points, each strictly inside (start, end). */
export function isValidSplitCutPoints(
  startIso: string,
  endIso: string,
  cutPointsIso: string[],
): boolean {
  if (cutPointsIso.length === 0) {
    return false;
  }

  const start = toDate(startIso);
  const end = toDate(endIso);
  if (!start || !end) {
    return false;
  }

  let prev = start.getTime();
  const endMs = end.getTime();

  for (const raw of cutPointsIso) {
    const cut = toDate(raw);
    if (!cut) {
      return false;
    }
    const cutMs = cut.getTime();
    if (cutMs <= prev || cutMs >= endMs) {
      return false;
    }
    prev = cutMs;
  }

  return true;
}
