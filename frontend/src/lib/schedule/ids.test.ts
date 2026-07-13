import { describe, expect, it } from "vitest";
import {
  allDayChipId,
  buildTimeEntriesByItemId,
  eventItemId,
  timeEntryItemId,
  isEventItemId,
  isTimeEntryItemId,
  parseEventItemId,
  parseTimeEntryItemId,
} from "./ids";

describe("schedule ids", () => {
  it("round-trips event and time-entry item ids", () => {
    expect(eventItemId(34)).toBe("event-34");
    expect(parseEventItemId("event-34")).toBe(34);
    expect(isEventItemId("event-34")).toBe(true);
    expect(isEventItemId("time-entry-34")).toBe(false);

    expect(timeEntryItemId(21)).toBe("time-entry-21");
    expect(parseTimeEntryItemId("time-entry-21")).toBe(21);
    expect(isTimeEntryItemId("time-entry-21")).toBe(true);
    expect(isTimeEntryItemId("event-21")).toBe(false);
  });

  it("builds all-day chip ids and time-entry lookup maps", () => {
    expect(allDayChipId(7, "2026-07-03")).toBe("event-7@2026-07-03");

    const timeEntries = [
      {
        id: 11,
        periodId: 1,
        localWorkDate: "2026-07-02",
        start: "2026-07-02T09:00:00Z",
        end: "2026-07-02T10:00:00Z",
        durationMinutes: 60,
        categoryId: 10,
        description: "",
        attestation: "confirmed",
        method: undefined,
        workType: "worked",
        billableStatus: "unset",
      },
    ] as const;

    const byItemId = buildTimeEntriesByItemId([...timeEntries]);
    expect(byItemId.get("time-entry-11")).toEqual(timeEntries[0]);
  });
});
