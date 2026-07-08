// @vitest-environment jsdom

import { renderHook, act } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { GapSuggestion } from "@/lib/api";
import { useScheduleGapSuggest } from "./useScheduleGapSuggest";

describe("useScheduleGapSuggest", () => {
  it("requests AI suggestion when selecting a gap", () => {
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
    const createMutate = vi.fn();

    const { result } = renderHook(() =>
      useScheduleGapSuggest({
        activePeriodId: 12,
        aiConfigured: true,
        suggestGapFillMutation: {
          isPending: false,
          error: null,
          mutate: suggestMutate,
          reset: vi.fn(),
        },
        createGapFillMutation: {
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
    expect(result.current.selectedGap?.day).toBe("2026-07-08");
    expect(result.current.gapSuggestion).toEqual({
      category: "Deep Work",
      description: "Focus block",
      evidenceCount: 3,
    });
  });

  it("creates gap fill and closes dialog on confirm", () => {
    const suggestReset = vi.fn();
    const createMutate = vi.fn(
      (_payload: unknown, options?: { onSuccess?: () => void }) => {
        options?.onSuccess?.();
      },
    );

    const { result } = renderHook(() =>
      useScheduleGapSuggest({
        activePeriodId: 10,
        aiConfigured: false,
        suggestGapFillMutation: {
          isPending: false,
          error: null,
          mutate: vi.fn(),
          reset: suggestReset,
        },
        createGapFillMutation: {
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
        note: "Deep work",
      });
    });

    expect(createMutate).toHaveBeenCalledWith(
      expect.objectContaining({
        periodId: 10,
        day: "2026-07-09",
        startMinutes: 480,
        endMinutes: 540,
        categoryId: 4,
        note: "Deep work",
      }),
      expect.any(Object),
    );
    expect(result.current.gapSuggestOpen).toBe(false);
    expect(suggestReset).toHaveBeenCalledTimes(1);
  });
});
