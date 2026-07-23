import type { TzSegment } from "@/lib/api";

export function toDate(value: string | undefined) {
  if (!value) {
    return null;
  }

  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? null : date;
}

export function defaultTimeZone() {
  return Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
}

export function zonedDateTimeParts(date: Date, timeZone: string) {
  const parts = new Intl.DateTimeFormat("en-CA", {
    day: "2-digit",
    hour: "2-digit",
    hourCycle: "h23",
    minute: "2-digit",
    month: "2-digit",
    timeZone,
    year: "numeric",
  }).formatToParts(date);
  const values = Object.fromEntries(
    parts
      .filter((part) => part.type !== "literal")
      .map((part) => [part.type, part.value]),
  );

  return {
    day: `${values.year}-${values.month}-${values.day}`,
    minutes: Number(values.hour) * 60 + Number(values.minute),
  };
}

/** Convert a local calendar day + minute-of-day in `timeZone` to a UTC ISO instant. */
export function zonedDayMinutesToIso(
  day: string,
  minutes: number,
  timeZone: string,
): string | null {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(day) || !Number.isInteger(minutes)) {
    return null;
  }
  if (minutes < 0 || minutes > 24 * 60) {
    return null;
  }

  const [year, month, date] = day.split("-").map(Number);
  // Search a ±36h UTC window around the nominal civil midnight.
  let lo = Date.UTC(year, month - 1, date, 0, 0, 0) - 36 * 60 * 60 * 1000;
  let hi = Date.UTC(year, month - 1, date, 0, 0, 0) + 36 * 60 * 60 * 1000;

  while (lo <= hi) {
    const mid = Math.floor((lo + hi) / 2);
    // Snap candidate to whole minutes so we match zonedDateTimeParts granularity.
    const snapped = mid - (mid % 60_000);
    const parts = zonedDateTimeParts(new Date(snapped), timeZone);
    const cmp =
      parts.day < day
        ? -1
        : parts.day > day
          ? 1
          : parts.minutes - minutes;

    if (cmp === 0) {
      return new Date(snapped).toISOString();
    }
    if (cmp < 0) {
      lo = mid + 60_000;
    } else {
      hi = mid - 60_000;
    }
  }

  return null;
}

export function activeTimeZoneForDay(day: string, tzSegments: TzSegment[]) {
  if (tzSegments.length === 0) {
    return defaultTimeZone();
  }

  let activeSegment = tzSegments[0];
  for (const segment of tzSegments) {
    if (segment.effectiveFromDate <= day) {
      activeSegment = segment;
    } else {
      break;
    }
  }

  return activeSegment.ianaTz;
}

export function zonedPosition(
  value: string | undefined,
  tzSegments: TzSegment[],
) {
  const date = toDate(value);

  if (!date) {
    return null;
  }

  const initialTimeZone = tzSegments[0]?.ianaTz ?? defaultTimeZone();
  const initialParts = zonedDateTimeParts(date, initialTimeZone);
  const activeTimeZone = activeTimeZoneForDay(initialParts.day, tzSegments);

  if (activeTimeZone === initialTimeZone) {
    return initialParts;
  }

  return zonedDateTimeParts(date, activeTimeZone);
}
