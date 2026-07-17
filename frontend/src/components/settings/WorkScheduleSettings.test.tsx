// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ScheduleException, WorkSchedule } from "@/lib/api/types";
import { WorkScheduleSettings } from "./WorkScheduleSettings";

const mocks = vi.hoisted(() => ({
  replaceMutate: vi.fn(),
  upsertExceptionMutate: vi.fn(),
  deleteExceptionMutate: vi.fn(),
  schedule: null as WorkSchedule | null | undefined,
  exceptions: [] as ScheduleException[],
  schedulesLoading: false,
  exceptionsLoading: false,
}));

vi.mock("@/lib/api", () => ({
  useActiveWorkSchedule: () => ({
    data: mocks.schedule,
    isLoading: mocks.schedulesLoading,
  }),
  useScheduleExceptions: () => ({
    data: mocks.exceptions,
    isLoading: mocks.exceptionsLoading,
  }),
  useReplaceActiveWorkSchedule: () => ({
    isPending: false,
    mutateAsync: mocks.replaceMutate,
  }),
  useUpsertScheduleException: () => ({
    isPending: false,
    mutateAsync: mocks.upsertExceptionMutate,
  }),
  useDeleteScheduleException: () => ({
    isPending: false,
    mutateAsync: mocks.deleteExceptionMutate,
  }),
}));

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });

  function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  }

  return Wrapper;
}

const seedSchedule: WorkSchedule = {
  id: 1,
  timezone: "America/Toronto",
  workweekStart: "monday",
  effectiveFrom: "1970-01-01",
  days: [
    { weekday: "monday", expectedMinutes: 480, windows: [{ startMinutes: 540, endMinutes: 1020 }] },
    { weekday: "tuesday", expectedMinutes: 480, windows: [{ startMinutes: 540, endMinutes: 1020 }] },
    { weekday: "wednesday", expectedMinutes: 480, windows: [{ startMinutes: 540, endMinutes: 1020 }] },
    { weekday: "thursday", expectedMinutes: 480, windows: [{ startMinutes: 540, endMinutes: 1020 }] },
    { weekday: "friday", expectedMinutes: 480, windows: [{ startMinutes: 540, endMinutes: 1020 }] },
    { weekday: "saturday", expectedMinutes: 0, windows: [] },
    { weekday: "sunday", expectedMinutes: 0, windows: [] },
  ],
};

describe("WorkScheduleSettings", () => {
  beforeEach(() => {
    cleanup();
    Element.prototype.scrollIntoView = vi.fn();
    mocks.schedule = structuredClone(seedSchedule);
    mocks.exceptions = [];
    mocks.schedulesLoading = false;
    mocks.exceptionsLoading = false;
    mocks.replaceMutate.mockReset().mockResolvedValue(seedSchedule);
    mocks.upsertExceptionMutate.mockReset().mockResolvedValue({
      id: 1,
      date: "2026-07-04",
      kind: "holiday",
      expectedMinutes: 0,
      windows: [],
    });
    mocks.deleteExceptionMutate.mockReset().mockResolvedValue(undefined);
  });

  it("loads active schedule timezone, workweek start, and weekday hours", () => {
    render(<WorkScheduleSettings />, { wrapper: createWrapper() });

    expect(screen.getByLabelText("Timezone")).toHaveProperty(
      "value",
      "America/Toronto",
    );
    expect(screen.getByLabelText("Workweek starts")).toBeTruthy();
    expect(screen.getByLabelText("Monday hours")).toHaveProperty("value", "8");
    expect(screen.getByLabelText("Saturday hours")).toHaveProperty("value", "0");
  });

  it("saves the weekday template via replaceActiveWorkSchedule", async () => {
    render(<WorkScheduleSettings />, { wrapper: createWrapper() });

    const mondayHours = screen.getByLabelText("Monday hours");
    fireEvent.change(mondayHours, { target: { value: "7.5" } });
    fireEvent.blur(mondayHours);
    fireEvent.click(screen.getByRole("button", { name: "Save schedule" }));

    expect(mocks.replaceMutate).toHaveBeenCalledTimes(1);
    const input = mocks.replaceMutate.mock.calls[0][0];
    expect(input.timezone).toBe("America/Toronto");
    expect(input.workweekStart).toBe("monday");
    expect(input.effectiveFrom).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    expect(input.days).toHaveLength(7);
    expect(input.days.find((d: { weekday: string }) => d.weekday === "monday")).toEqual(
      expect.objectContaining({
        weekday: "monday",
        expectedMinutes: 450,
      }),
    );
  });

  it("lists exceptions and can add a holiday", async () => {
    mocks.exceptions = [
      {
        id: 9,
        date: "2026-01-01",
        kind: "holiday",
        expectedMinutes: 0,
        windows: [],
      },
    ];

    render(<WorkScheduleSettings />, { wrapper: createWrapper() });

    expect(screen.getByText("2026-01-01")).toBeTruthy();
    expect(screen.getByText("holiday")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "Add exception" }));
    const dialog = screen.getByRole("dialog");
    fireEvent.change(within(dialog).getByLabelText("Date"), {
      target: { value: "2026-07-04" },
    });
    fireEvent.click(within(dialog).getByRole("button", { name: "Save exception" }));

    expect(mocks.upsertExceptionMutate).toHaveBeenCalledWith(
      expect.objectContaining({
        date: "2026-07-04",
        kind: "holiday",
        expectedMinutes: 0,
      }),
    );
  });

  it("deletes an exception by date", async () => {
    mocks.exceptions = [
      {
        id: 9,
        date: "2026-01-01",
        kind: "holiday",
        expectedMinutes: 0,
        windows: [],
      },
    ];

    render(<WorkScheduleSettings />, { wrapper: createWrapper() });

    fireEvent.click(screen.getByRole("button", { name: "Delete exception 2026-01-01" }));

    expect(mocks.deleteExceptionMutate).toHaveBeenCalledWith("2026-01-01");
  });
});
