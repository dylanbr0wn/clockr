// @vitest-environment jsdom

import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useSchedulePage } from "./useSchedulePage";

const mockViewDayCountState = vi.hoisted(() => ({
  value: 7,
  setValue: vi.fn(),
}));

const mockState = vi.hoisted(() => {
  const createMutate = vi.fn();
  const createGapTimeEntryMutate = vi.fn();
  const suggestMutate = vi.fn();
  const evidenceMutate = vi.fn();
  const updateMutate = vi.fn();
  const deleteMutate = vi.fn();
  const excludeMutate = vi.fn();

  return {
    periods: [
      { id: 1, startDate: "2026-07-01", endDate: "2026-07-14" },
      { id: 2, startDate: "2026-07-15", endDate: "2026-07-28" },
    ],
    currentPeriod: { id: 1, startDate: "2026-07-01", endDate: "2026-07-14" },
    categories: [{
      id: 10,
      name: "Work",
      description: "",
      key: "Work",
      color: "#0EA5E9",
      isDefaultGap: false,
      archived: false,
      inUse: false,
    }],
    projects: [],
    events: [],
    timeEntries: [
      {
        id: 11,
        periodId: 1,
        localWorkDate: "2026-07-02",
        start: "2026-07-02T09:00:00Z",
        end: "2026-07-02T10:00:00Z",
        durationMinutes: 60,
        categoryId: 10,
        description: "Manual description",
        attestation: "confirmed",
        workType: "worked",
        billableStatus: "unset",
      },
      {
        id: 12,
        periodId: 1,
        localWorkDate: "2026-07-02",
        start: "2026-07-02T10:00:00Z",
        end: "2026-07-02T11:00:00Z",
        durationMinutes: 60,
        categoryId: 10,
        description: "auto one",
        attestation: "confirmed",
        method: "gap_fill",
        workType: "worked",
        billableStatus: "unset",
      },
    ],
    gapTimeline: [],
    reviewDecisions: [],
    tzSegments: [],
    aiConfigured: true,
    aiLocal: false,
    createMutate,
    createGapTimeEntryMutate,
    suggestMutate,
    evidenceMutate,
    updateMutate,
    deleteMutate,
    excludeMutate,
  };
});

vi.mock("../settings/useJsonSetting", () => ({
  useJsonSetting: (key: string) => {
    if (key === "privacy.fields") {
      return {
        value: {
          title: true,
          attendees: true,
          description: false,
          location: false,
        },
        setValue: vi.fn(),
        isLoading: false,
        isSaving: false,
        error: null,
      };
    }

    return {
      value: mockViewDayCountState.value,
      setValue: mockViewDayCountState.setValue,
      isLoading: false,
      isSaving: false,
      error: null,
    };
  },
}));

vi.mock("@/lib/schedule", async () => {
  const actual = await vi.importActual<typeof import("@/lib/schedule")>("@/lib/schedule");
  return {
    ...actual,
    localDateKey: () => "2026-07-02",
    defaultTimeZone: () => "UTC",
  };
});

vi.mock("@/lib/api", () => ({
  usePeriods: () => ({ data: mockState.periods, isLoading: false, error: null }),
  useCurrentPeriod: () => ({
    data: mockState.currentPeriod,
    isLoading: false,
    error: null,
  }),
  useCategories: () => ({ data: mockState.categories, isLoading: false, error: null }),
  useProjects: () => ({ data: mockState.projects, isLoading: false, error: null }),
  useEvents: () => ({ data: mockState.events, isLoading: false, error: null }),
  useEventCategoryOverlays: () => ({
    data: [],
    isLoading: false,
    error: null,
  }),
  useTimeEntries: () => ({ data: mockState.timeEntries, isLoading: false, error: null }),
  useGapTimeline: () => ({ data: mockState.gapTimeline, isLoading: false, error: null }),
  useReviewDecisions: () => ({
    data: mockState.reviewDecisions,
    isLoading: false,
    error: null,
  }),
  useTzSegments: () => ({ data: mockState.tzSegments, isLoading: false, error: null }),
  useCreateTimeEntry: () => ({
    mutate: mockState.createMutate,
    isPending: false,
    error: null,
  }),
  useCreateGapTimeEntry: () => ({
    mutate: mockState.createGapTimeEntryMutate,
    isPending: false,
    error: null,
  }),
  useSuggestGapFill: () => ({
    mutate: mockState.suggestMutate,
    isPending: false,
    error: null,
    reset: vi.fn(),
  }),
  useListGapEvidence: () => ({
    mutate: mockState.evidenceMutate,
    isPending: false,
    error: null,
    reset: vi.fn(),
  }),
  useUpdateTimeEntry: () => ({
    mutate: mockState.updateMutate,
    isPending: false,
    error: null,
  }),
  useDeleteTimeEntry: () => ({
    mutate: mockState.deleteMutate,
    isPending: false,
    error: null,
  }),
  useExcludeEvent: () => ({
    mutate: mockState.excludeMutate,
    isPending: false,
    error: null,
  }),
  useAIConfigured: () => ({ isConfigured: mockState.aiConfigured, baseURL: "http://local" }),
  useClassifyAIEndpoint: () => ({ data: { local: mockState.aiLocal } }),
}));

describe("useSchedulePage", () => {
  beforeEach(() => {
    mockViewDayCountState.value = 7;
    mockViewDayCountState.setValue.mockReset();
    mockState.createMutate.mockReset();
    mockState.createGapTimeEntryMutate.mockReset();
    mockState.suggestMutate.mockReset();
    mockState.evidenceMutate.mockReset();
    mockState.updateMutate.mockReset();
    mockState.deleteMutate.mockReset();
    mockState.excludeMutate.mockReset();
    mockState.currentPeriod = { id: 1, startDate: "2026-07-01", endDate: "2026-07-14" };
  });

  it("loads persisted view day count from settings", async () => {
    mockViewDayCountState.value = 14;

    const { result } = renderHook(() => useSchedulePage());
    await waitFor(() => expect(result.current.selectedPeriodId).toBe(1));

    expect(result.current.viewDayCount).toBe(14);
  });

  it("persists view day count when toggled", async () => {
    const { result } = renderHook(() => useSchedulePage());
    await waitFor(() => expect(result.current.selectedPeriodId).toBe(1));

    act(() => {
      result.current.setViewDayCount(1);
    });

    expect(mockViewDayCountState.setValue).toHaveBeenCalledWith(1);
  });

  it("falls back to default view day count for invalid persisted values", async () => {
    mockViewDayCountState.value = 99;

    const { result } = renderHook(() => useSchedulePage());
    await waitFor(() => expect(result.current.selectedPeriodId).toBe(1));

    expect(result.current.viewDayCount).toBe(7);
  });

  it("selects current period on initial load", async () => {
    const { result } = renderHook(() => useSchedulePage());
    await waitFor(() => expect(result.current.selectedPeriodId).toBe(1));
    expect(result.current.activePeriodId).toBe(1);
  });

  it("resets review queue and create flow when period changes", async () => {
    const { result } = renderHook(() => useSchedulePage());
    await waitFor(() => expect(result.current.selectedPeriodId).toBe(1));

    act(() => {
      result.current.setReviewQueueOpen(true);
      result.current.handleCreate({
        day: "2026-07-02",
        startMinutes: 300,
        endMinutes: 360,
      });
    });
    expect(result.current.reviewQueueOpen).toBe(true);
    expect(result.current.editingEvent).not.toBeNull();

    act(() => {
      result.current.setSelectedPeriodId(2);
    });

    await waitFor(() => expect(result.current.activePeriodId).toBe(2));
    await waitFor(() => expect(result.current.reviewQueueOpen).toBe(false));
    expect(result.current.editingEvent).toBeNull();
  });

  it("sends expected payloads for reset-day and commit handlers", async () => {
    const { result } = renderHook(() => useSchedulePage());
    await waitFor(() => expect(result.current.activePeriodId).toBe(1));

    act(() => {
      result.current.handleResetDay("2026-07-02");
    });
    expect(mockState.deleteMutate).toHaveBeenCalledTimes(1);
    expect(mockState.deleteMutate).toHaveBeenCalledWith({
      id: 11,
      periodId: 1,
    });

    act(() => {
      result.current.handleCommit({
        itemId: "time-entry-11",
        day: "2026-07-02",
        startMinutes: 560,
        endMinutes: 620,
        interaction: "move",
        item: {
          id: "time-entry-11",
          day: "2026-07-02",
          startMinutes: 540,
          endMinutes: 600,
        },
      });
    });

    expect(mockState.updateMutate).toHaveBeenCalledTimes(1);
    expect(mockState.updateMutate).toHaveBeenCalledWith(
      expect.objectContaining({
        id: 11,
        periodId: 1,
        day: "2026-07-02",
        startMinutes: 560,
        endMinutes: 620,
        description: "Manual description",
      }),
      expect.objectContaining({
        onSettled: expect.any(Function),
      }),
    );
  });

  it("excludes calendar timed items and all-day chips", async () => {
    const { result } = renderHook(() => useSchedulePage());
    await waitFor(() => expect(result.current.activePeriodId).toBe(1));

    act(() => {
      result.current.handleExcludeEvent({
        id: "event-42",
        day: "2026-07-02",
        startMinutes: 540,
        endMinutes: 600,
        metadata: {
          title: "Standup",
          category: "Work",
          kind: "calendar",
        },
      });
    });
    expect(mockState.excludeMutate).toHaveBeenCalledWith({
      eventId: 42,
      periodId: 1,
    });

    mockState.excludeMutate.mockClear();
    act(() => {
      result.current.handleExcludeAllDayChip({
        id: "event-7-2026-07-03",
        eventId: 7,
        day: "2026-07-03",
        title: "Holiday",
        category: "Work",
        kind: "calendar",
        allDaySpan: "single",
      });
    });
    expect(mockState.excludeMutate).toHaveBeenCalledWith({
      eventId: 7,
      periodId: 1,
    });

    mockState.excludeMutate.mockClear();
    act(() => {
      result.current.handleExcludeAllDayChip({
        id: "event-8-2026-07-04",
        eventId: 8,
        day: "2026-07-04",
        title: "PTO",
        category: "Needs review",
        kind: "review",
        allDaySpan: "single",
      });
    });
    expect(mockState.excludeMutate).toHaveBeenCalledWith({
      eventId: 8,
      periodId: 1,
    });
  });
});
