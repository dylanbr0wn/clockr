// @vitest-environment jsdom

import { renderHook, act } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { GapEvidenceItem, GapSuggestion } from "@/lib/api";
import { useScheduleGapSuggest } from "./useScheduleGapSuggest";

function createMutations({
  suggestMutate = vi.fn(),
  evidenceMutate = vi.fn(),
  suggestReset = vi.fn(),
  evidenceReset = vi.fn(),
}: {
  suggestMutate?: ReturnType<typeof vi.fn>;
  evidenceMutate?: ReturnType<typeof vi.fn>;
  suggestReset?: ReturnType<typeof vi.fn>;
  evidenceReset?: ReturnType<typeof vi.fn>;
} = {}) {
  return {
    suggestGapFillMutation: {
      isPending: false,
      error: null,
      mutate: suggestMutate,
      reset: suggestReset,
    },
    listGapEvidenceMutation: {
      isPending: false,
      error: null,
      mutate: evidenceMutate,
      reset: evidenceReset,
    },
  };
}

describe("useScheduleGapSuggest", () => {
  it("requests AI suggestion and evidence when selecting a gap", () => {
    const suggestMutate = vi.fn(
      (
        _payload: unknown,
        options?: { onSuccess?: (value: GapSuggestion) => void },
      ) => {
        options?.onSuccess?.({
          category: "Deep Work",
          description: "Focus block",
          evidenceCount: 3,
        });
      },
    );
    const evidenceMutate = vi.fn(
      (
        _payload: unknown,
        options?: { onSuccess?: (value: GapEvidenceItem[]) => void },
      ) => {
        options?.onSuccess?.([
          {
            provider: "github",
            kind: "commit",
            summary: "Merged PR #42",
            source: "acme/widget",
          },
        ]);
      },
    );
    const createMutate = vi.fn();

    const { result } = renderHook(() =>
      useScheduleGapSuggest({
        activePeriodId: 12,
        aiConfigured: true,
        ...createMutations({ suggestMutate, evidenceMutate }),
        createGapTimeEntryMutation: {
          isPending: false,
          mutate: createMutate,
        },
        resetKey: 12,
      }),
    );

    act(() => {
      result.current.handleSelectGap({
        id: "g1",
        day: "2026-07-08",
        startMinutes: 540,
        endMinutes: 600,
        gapWindowStart: "2026-07-08T09:00:00Z",
        gapWindowEnd: "2026-07-08T10:00:00Z",
      });
    });

    expect(suggestMutate).toHaveBeenCalledTimes(1);
    expect(evidenceMutate).toHaveBeenCalledTimes(1);
    expect(result.current.selectedGap?.day).toBe("2026-07-08");
    expect(result.current.gapSuggestion).toEqual({
      category: "Deep Work",
      description: "Focus block",
      evidenceCount: 3,
    });
    expect(result.current.gapEvidenceItems).toEqual([
      {
        provider: "github",
        kind: "commit",
        summary: "Merged PR #42",
        source: "acme/widget",
      },
    ]);
  });

  it("creates gap time entry and closes dialog on confirm", () => {
    const suggestReset = vi.fn();
    const evidenceReset = vi.fn();
    const createMutate = vi.fn(
      (_payload: unknown, options?: { onSuccess?: () => void }) => {
        options?.onSuccess?.();
      },
    );

    const { result } = renderHook(() =>
      useScheduleGapSuggest({
        activePeriodId: 10,
        aiConfigured: false,
        ...createMutations({ suggestReset, evidenceReset }),
        createGapTimeEntryMutation: {
          isPending: false,
          mutate: createMutate,
        },
        resetKey: 10,
      }),
    );

    act(() => {
      result.current.handleSelectGap({
        id: "g2",
        day: "2026-07-09",
        startMinutes: 480,
        endMinutes: 540,
        gapWindowStart: "2026-07-09T08:00:00Z",
        gapWindowEnd: "2026-07-09T09:00:00Z",
      });
    });

    act(() => {
      result.current.handleConfirmGapSuggest({
        categoryId: 4,
        description: "Deep work",
        workType: "worked",
        projectId: 12,
        billableStatus: "billable",
      });
    });

    expect(createMutate).toHaveBeenCalledWith(
      expect.objectContaining({
        periodId: 10,
        day: "2026-07-09",
        startMinutes: 480,
        endMinutes: 540,
        categoryId: 4,
        description: "Deep work",
        workType: "worked",
        projectId: 12,
        billableStatus: "billable",
      }),
      expect.any(Object),
    );
    expect(result.current.gapSuggestOpen).toBe(false);
    expect(suggestReset).toHaveBeenCalledTimes(1);
    expect(evidenceReset).toHaveBeenCalledTimes(1);
  });
});
