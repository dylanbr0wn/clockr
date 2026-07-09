import { describe, expect, it } from "vitest";
import type { AllDayChip } from "@/lib/schedule";
import {
  allDaySpanClasses,
  resolveVisibleAllDaySpan,
} from "./allDaySpan";

function chip(
  eventId: number,
  day: string,
  allDaySpan: AllDayChip["allDaySpan"] = "single",
): AllDayChip {
  return {
    id: `event-${eventId}@${day}`,
    eventId,
    day,
    title: "Event",
    category: "Work",
    kind: "calendar",
    allDaySpan,
  };
}

describe("allDaySpanClasses", () => {
  it("keeps single-day chips pill-shaped with inset and shadow", () => {
    expect(allDaySpanClasses("single")).toContain("rounded-md");
    expect(allDaySpanClasses("single")).toContain("mx-1");
    expect(allDaySpanClasses("single")).toContain("shadow-sm");
  });

  it("rounds only the outer ends of multi-day segments", () => {
    expect(allDaySpanClasses("start")).toContain("rounded-l-md");
    expect(allDaySpanClasses("start")).toContain("rounded-r-none");
    expect(allDaySpanClasses("start")).not.toContain("rounded-r-sm");

    expect(allDaySpanClasses("middle")).toContain("rounded-none");

    expect(allDaySpanClasses("end")).toContain("rounded-r-md");
    expect(allDaySpanClasses("end")).toContain("rounded-l-none");
    expect(allDaySpanClasses("end")).not.toContain("rounded-l-sm");
  });

  it("removes shared-edge borders so abutting segments form one strip", () => {
    expect(allDaySpanClasses("start")).toContain("border-r-0");
    expect(allDaySpanClasses("start")).toContain("ml-1");
    expect(allDaySpanClasses("start")).not.toContain("-mr-px");

    expect(allDaySpanClasses("middle")).toContain("border-x-0");
    expect(allDaySpanClasses("middle")).not.toContain("-mx-px");

    expect(allDaySpanClasses("end")).toContain("border-l-0");
    expect(allDaySpanClasses("end")).toContain("mr-1");
    expect(allDaySpanClasses("end")).not.toContain("-ml-px");
  });
});

describe("resolveVisibleAllDaySpan", () => {
  const days = [
    { date: "2026-06-09" },
    { date: "2026-06-10" },
    { date: "2026-06-11" },
  ];

  it("treats an isolated chip as single", () => {
    const chipsByDay = new Map([["2026-06-10", [chip(1, "2026-06-10")]]]);

    expect(
      resolveVisibleAllDaySpan(chip(1, "2026-06-10"), 1, days, chipsByDay),
    ).toBe("single");
  });

  it("marks visible outer ends when the full span is clipped by the window", () => {
    const chipsByDay = new Map([
      ["2026-06-09", [chip(1, "2026-06-09", "middle")]],
      ["2026-06-10", [chip(1, "2026-06-10", "middle")]],
      ["2026-06-11", [chip(1, "2026-06-11", "middle")]],
    ]);

    expect(
      resolveVisibleAllDaySpan(
        chip(1, "2026-06-09", "middle"),
        0,
        days,
        chipsByDay,
      ),
    ).toBe("start");
    expect(
      resolveVisibleAllDaySpan(
        chip(1, "2026-06-10", "middle"),
        1,
        days,
        chipsByDay,
      ),
    ).toBe("middle");
    expect(
      resolveVisibleAllDaySpan(
        chip(1, "2026-06-11", "middle"),
        2,
        days,
        chipsByDay,
      ),
    ).toBe("end");
  });

  it("does not connect different events on adjacent days", () => {
    const chipsByDay = new Map([
      ["2026-06-09", [chip(1, "2026-06-09")]],
      ["2026-06-10", [chip(2, "2026-06-10")]],
    ]);

    expect(
      resolveVisibleAllDaySpan(chip(1, "2026-06-09"), 0, days, chipsByDay),
    ).toBe("single");
  });
});
