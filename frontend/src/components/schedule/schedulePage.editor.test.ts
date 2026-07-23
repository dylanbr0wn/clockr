// @vitest-environment jsdom

import { act, renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { TimeEntry } from "@/lib/api";
import { useSchedulePageEditor } from "./schedulePage.editor";

const timeEntries: TimeEntry[] = [
  {
    id: 11,
    periodId: 1,
    localWorkDate: "2026-07-02",
    start: "2026-07-02T09:00:00Z",
    end: "2026-07-02T10:00:00Z",
    durationMinutes: 60,
    categoryId: 10,
    description: "Existing description",
    attestation: "confirmed",
    workType: "worked",
    billableStatus: "unset",
  },
];

describe("useSchedulePageEditor", () => {
  it("passes description when saving event edits", () => {
    const createMutate = vi.fn();
    const updateMutate = vi.fn();

    const { result } = renderHook(() =>
      useSchedulePageEditor({
        activePeriodId: 1,
        timeEntries,
        createTimeEntryMutation: { mutate: createMutate },
        updateTimeEntryMutation: { mutate: updateMutate },
        adjustDraftTimeEntryMutation: { mutate: vi.fn() },
        deleteTimeEntryMutation: { mutate: vi.fn() },
        excludeEventMutation: { mutate: vi.fn() },
      }),
    );

    act(() => {
      result.current.handleCreate({
        day: "2026-07-03",
        startMinutes: 540,
        endMinutes: 600,
      });
    });

    act(() => {
      result.current.handleSaveEventEdit({
        day: "2026-07-03",
        startMinutes: 540,
        endMinutes: 600,
        categoryId: 10,
        note: "New title",
        description: "New description",
        workType: "worked",
        billableStatus: "unset",
      });
    });

    expect(createMutate).toHaveBeenCalledWith(
      expect.objectContaining({
        periodId: 1,
        description: "New description",
      }),
      expect.any(Object),
    );

    act(() => {
      result.current.handleOpenEventEditor({
        id: "time-entry-11",
        day: "2026-07-02",
        startMinutes: 540,
        endMinutes: 600,
        metadata: {
          title: "Title",
          category: "Work",
          kind: "manual",
          mutable: true,
        },
      });
    });

    act(() => {
      result.current.handleSaveEventEdit({
        day: "2026-07-02",
        startMinutes: 540,
        endMinutes: 600,
        categoryId: 10,
        note: "Updated title",
        description: "Updated description",
        workType: "worked",
        billableStatus: "unset",
      });
    });

    expect(updateMutate).toHaveBeenCalledWith(
      expect.objectContaining({
        id: 11,
        description: "Updated description",
      }),
      expect.any(Object),
    );
  });

  it("passes allocation fields when creating and updating time entries", () => {
    const createMutate = vi.fn();
    const updateMutate = vi.fn();

    const { result } = renderHook(() =>
      useSchedulePageEditor({
        activePeriodId: 1,
        timeEntries,
        createTimeEntryMutation: { mutate: createMutate },
        updateTimeEntryMutation: { mutate: updateMutate },
        adjustDraftTimeEntryMutation: { mutate: vi.fn() },
        deleteTimeEntryMutation: { mutate: vi.fn() },
        excludeEventMutation: { mutate: vi.fn() },
      }),
    );

    act(() => {
      result.current.handleCreate({
        day: "2026-07-03",
        startMinutes: 540,
        endMinutes: 600,
      });
    });

    act(() => {
      result.current.handleSaveEventEdit({
        day: "2026-07-03",
        startMinutes: 540,
        endMinutes: 600,
        categoryId: 10,
        note: "New title",
        description: "New description",
        workType: "worked",
        projectId: 7,
        billableStatus: "billable",
      });
    });

    expect(createMutate).toHaveBeenCalledWith(
      expect.objectContaining({
        workType: "worked",
        projectId: 7,
        billableStatus: "billable",
      }),
      expect.any(Object),
    );

    act(() => {
      result.current.handleOpenEventEditor({
        id: "time-entry-11",
        day: "2026-07-02",
        startMinutes: 540,
        endMinutes: 600,
        metadata: {
          title: "Title",
          category: "Work",
          kind: "manual",
          mutable: true,
        },
      });
    });

    act(() => {
      result.current.handleSaveEventEdit({
        day: "2026-07-02",
        startMinutes: 540,
        endMinutes: 600,
        categoryId: 10,
        note: "Updated title",
        description: "Updated description",
        workType: "paid_leave",
        billableStatus: "unset",
      });
    });

    expect(updateMutate).toHaveBeenCalledWith(
      expect.objectContaining({
        id: 11,
        workType: "paid_leave",
        billableStatus: "unset",
      }),
      expect.any(Object),
    );
    expect(updateMutate.mock.calls.at(-1)?.[0]).not.toHaveProperty("projectId");
  });

  it("preserves allocation fields when committing drag changes", () => {
    const updateMutate = vi.fn();
    const allocated: TimeEntry[] = [
      {
        ...timeEntries[0],
        workType: "worked",
        projectId: 9,
        billableStatus: "billable",
      },
    ];

    const { result } = renderHook(() =>
      useSchedulePageEditor({
        activePeriodId: 1,
        timeEntries: allocated,
        createTimeEntryMutation: { mutate: vi.fn() },
        updateTimeEntryMutation: { mutate: updateMutate },
        adjustDraftTimeEntryMutation: { mutate: vi.fn() },
        deleteTimeEntryMutation: { mutate: vi.fn() },
        excludeEventMutation: { mutate: vi.fn() },
      }),
    );

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

    expect(updateMutate).toHaveBeenCalledWith(
      expect.objectContaining({
        id: 11,
        workType: "worked",
        projectId: 9,
        billableStatus: "billable",
      }),
      expect.any(Object),
    );
  });

  it("preserves description when committing drag changes", () => {
    const updateMutate = vi.fn();

    const { result } = renderHook(() =>
      useSchedulePageEditor({
        activePeriodId: 1,
        timeEntries,
        createTimeEntryMutation: { mutate: vi.fn() },
        updateTimeEntryMutation: { mutate: updateMutate },
        adjustDraftTimeEntryMutation: { mutate: vi.fn() },
        deleteTimeEntryMutation: { mutate: vi.fn() },
        excludeEventMutation: { mutate: vi.fn() },
      }),
    );

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

    expect(updateMutate).toHaveBeenCalledWith(
      expect.objectContaining({
        id: 11,
        description: "Existing description",
      }),
      expect.any(Object),
    );
  });

  it("routes draft drag commits through adjustDraftTimeEntry", () => {
    const updateMutate = vi.fn();
    const adjustMutate = vi.fn();
    const draftEntries: TimeEntry[] = [
      {
        ...timeEntries[0],
        attestation: "draft",
      },
    ];

    const { result } = renderHook(() =>
      useSchedulePageEditor({
        activePeriodId: 1,
        timeEntries: draftEntries,
        createTimeEntryMutation: { mutate: vi.fn() },
        updateTimeEntryMutation: { mutate: updateMutate },
        adjustDraftTimeEntryMutation: { mutate: adjustMutate },
        deleteTimeEntryMutation: { mutate: vi.fn() },
        excludeEventMutation: { mutate: vi.fn() },
      }),
    );

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

    expect(updateMutate).not.toHaveBeenCalled();
    expect(adjustMutate).toHaveBeenCalledWith(
      expect.objectContaining({
        id: 11,
        startMinutes: 560,
        endMinutes: 620,
      }),
      expect.any(Object),
    );
  });

  it("opens draft editor separately from event editor", () => {
    const draftEntries: TimeEntry[] = [
      {
        ...timeEntries[0],
        attestation: "draft",
      },
    ];

    const { result } = renderHook(() =>
      useSchedulePageEditor({
        activePeriodId: 1,
        timeEntries: draftEntries,
        createTimeEntryMutation: { mutate: vi.fn() },
        updateTimeEntryMutation: { mutate: vi.fn() },
        adjustDraftTimeEntryMutation: { mutate: vi.fn() },
        deleteTimeEntryMutation: { mutate: vi.fn() },
        excludeEventMutation: { mutate: vi.fn() },
      }),
    );

    act(() => {
      result.current.handleOpenDraftEditor({
        id: "time-entry-11",
        day: "2026-07-02",
        startMinutes: 540,
        endMinutes: 600,
        metadata: {
          title: "Draft",
          category: "Work",
          kind: "draft",
          mutable: true,
          opensDraftEditor: true,
        },
      });
    });

    expect(result.current.draftingItemId).toBe("time-entry-11");
    expect(result.current.editingItemId).toBeNull();

    act(() => {
      result.current.handleOpenEventEditor({
        id: "time-entry-11",
        day: "2026-07-02",
        startMinutes: 540,
        endMinutes: 600,
        metadata: {
          title: "Draft",
          category: "Work",
          kind: "draft",
          mutable: true,
          opensDraftEditor: true,
        },
      });
    });

    expect(result.current.editingItemId).toBeNull();
  });
});
