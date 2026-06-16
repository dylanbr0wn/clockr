export interface Category {
  id: number;
  name: string;
  isDefaultGap: boolean;
}

export interface Calendar {
  id: number;
  googleCalendarId: string;
  name: string;
  isPrimary: boolean;
  selected: boolean;
  defaultCategoryId?: number;
}

export interface Period {
  id: number;
  startDate: string;
  endDate: string;
  cadence: string;
  anchorDate: string;
  targetHoursPerDay: number;
  lastSyncedAt?: string;
}

export interface Event {
  id: number;
  periodId: number;
  calendarId: number;
  googleEventId: string;
  instanceId?: string;
  recurringEventId?: string;
  icalUid?: string;
  title: string;
  description?: string;
  location?: string;
  organizer?: string;
  allDay: boolean;
  start?: string;
  end?: string;
  startDate?: string;
  endDate?: string;
  originalTz?: string;
  active: boolean;
}

export interface GapFill {
  id: number;
  periodId: number;
  day: string;
  start: string;
  end: string;
  categoryId?: number;
  note?: string;
  source: string;
}

export interface ManualEventInput {
  periodId: number;
  day: string;
  startMinutes: number;
  endMinutes: number;
  categoryId?: number;
  note?: string;
}

export interface ManualEventUpdateInput extends ManualEventInput {
  id: number;
}

export interface ManualEventDeleteInput {
  id: number;
  periodId: number;
}

export interface ManualEventResult {
  periodId: number;
  id: number;
}

export interface ReviewItem {
  id: number;
  periodId: number;
  kind: string;
  eventId?: number;
  payload: string;
  status: string;
}

export interface TzSegment {
  id: number;
  periodId: number;
  effectiveFromDate: string;
  ianaTz: string;
}

export interface Interval {
  start: string;
  end: string;
}

export interface DayTimeline {
  date: string;
  tz: string;
  windowStart: string;
  windowEnd: string;
  events: Interval[];
  filled: Interval[];
  gaps: Interval[];
  coveredHours: number;
  gapHours: number;
}
