// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useAppBootstrap } from "./app-bootstrap";
import { shietQueryKeys } from "@/lib/api";

const listPeriodsMock = vi.fn(async () => [
  { id: 1, startDate: "2026-07-01", endDate: "2026-07-14" },
]);
const ensureCurrentPeriodMock = vi.fn(
  async (_today: string, _ianaTz: string) => ({
    id: 1,
    startDate: "2026-07-01",
    endDate: "2026-07-14",
  }),
);
const listCategoriesMock = vi.fn(async () => [
  {
    id: 10,
    name: "Work",
    description: "",
    key: "Work",
    color: "#0EA5E9",
    isDefaultGap: false,
    archived: false,
    inUse: false,
  },
]);

vi.mock("@/lib/schedule", async () => {
  const actual = await vi.importActual<typeof import("@/lib/schedule")>(
    "@/lib/schedule",
  );
  return {
    ...actual,
    localDateKey: () => "2026-07-02",
    defaultTimeZone: () => "UTC",
  };
});

vi.mock("@/lib/api/shietService", () => ({
  listPeriods: () => listPeriodsMock(),
  ensureCurrentPeriod: (today: string, ianaTz: string) =>
    ensureCurrentPeriodMock(today, ianaTz),
  listCategories: () => listCategoriesMock(),
}));

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  });

  function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  }

  return { queryClient, Wrapper };
}

describe("useAppBootstrap", () => {
  beforeEach(() => {
    listPeriodsMock.mockClear();
    ensureCurrentPeriodMock.mockClear();
    listCategoriesMock.mockClear();
  });

  it("requests periods, current period, and categories without SchedulePage", async () => {
    const { queryClient, Wrapper } = createWrapper();

    renderHook(() => useAppBootstrap(), { wrapper: Wrapper });

    await waitFor(() => {
      expect(listPeriodsMock).toHaveBeenCalledTimes(1);
      expect(ensureCurrentPeriodMock).toHaveBeenCalledWith("2026-07-02", "UTC");
      expect(listCategoriesMock).toHaveBeenCalledTimes(1);
    });

    expect(queryClient.getQueryData(shietQueryKeys.periods())).toEqual([
      { id: 1, startDate: "2026-07-01", endDate: "2026-07-14" },
    ]);
    expect(
      queryClient.getQueryData(shietQueryKeys.currentPeriod("2026-07-02", "UTC")),
    ).toEqual({ id: 1, startDate: "2026-07-01", endDate: "2026-07-14" });
    expect(queryClient.getQueryData(shietQueryKeys.categories())).toEqual([
      {
        id: 10,
        name: "Work",
        description: "",
        key: "Work",
        color: "#0EA5E9",
        isDefaultGap: false,
        archived: false,
        inUse: false,
      },
    ]);
  });
});
