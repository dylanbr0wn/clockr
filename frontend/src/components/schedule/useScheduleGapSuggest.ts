import { useEffect, useState } from "react";
import type { GapSuggestion } from "@/lib/api";
import type { ScheduleGapOverlay } from "@/lib/schedule";
import type { SelectedGap } from "./GapSuggestDialog";

interface SuggestGapFillMutation {
  isPending: boolean;
  error: unknown;
  mutate: (
    payload: { start: string; end: string },
    options?: { onSuccess?: (suggestion: GapSuggestion) => void },
  ) => void;
  reset: () => void;
}

interface CreateGapFillMutation {
  isPending: boolean;
  mutate: (
    payload: {
      periodId: number;
      day: string;
      startMinutes: number;
      endMinutes: number;
      categoryId?: number;
      note: string;
    },
    options?: { onSuccess?: () => void },
  ) => void;
}

interface UseScheduleGapSuggestParams {
  activePeriodId: number | undefined;
  aiConfigured: boolean;
  suggestGapFillMutation: SuggestGapFillMutation;
  createGapFillMutation: CreateGapFillMutation;
  resetKey: number | undefined;
}

export function useScheduleGapSuggest({
  activePeriodId,
  aiConfigured,
  suggestGapFillMutation,
  createGapFillMutation,
  resetKey,
}: UseScheduleGapSuggestParams) {
  const [selectedGap, setSelectedGap] = useState<SelectedGap | null>(null);
  const [gapSuggestion, setGapSuggestion] = useState<GapSuggestion | null>(null);

  const requestGapSuggestion = (gap: SelectedGap) => {
    setGapSuggestion(null);
    suggestGapFillMutation.mutate(
      {
        start: gap.gapWindowStart,
        end: gap.gapWindowEnd,
      },
      {
        onSuccess: (suggestion) => {
          setGapSuggestion(suggestion);
        },
      },
    );
  };

  const handleSelectGap = (overlay: ScheduleGapOverlay) => {
    const gap: SelectedGap = {
      day: overlay.day,
      startMinutes: overlay.startMinutes,
      endMinutes: overlay.endMinutes,
      gapWindowStart: overlay.gapWindowStart,
      gapWindowEnd: overlay.gapWindowEnd,
    };
    setSelectedGap(gap);
    setGapSuggestion(null);

    if (aiConfigured) {
      requestGapSuggestion(gap);
    }
  };

  const handleCloseGapSuggest = () => {
    setSelectedGap(null);
    setGapSuggestion(null);
    suggestGapFillMutation.reset();
  };

  const handleRetryGapSuggest = () => {
    if (!selectedGap) {
      return;
    }
    requestGapSuggestion(selectedGap);
  };

  const handleConfirmGapSuggest = (values: { categoryId?: number; note: string }) => {
    if (!selectedGap || !activePeriodId) {
      return;
    }

    createGapFillMutation.mutate(
      {
        periodId: activePeriodId,
        day: selectedGap.day,
        startMinutes: selectedGap.startMinutes,
        endMinutes: selectedGap.endMinutes,
        categoryId: values.categoryId,
        note: values.note,
      },
      {
        onSuccess: () => {
          handleCloseGapSuggest();
        },
      },
    );
  };

  const resetGapSuggestState = () => {
    setSelectedGap(null);
    setGapSuggestion(null);
  };

  useEffect(() => {
    resetGapSuggestState();
  }, [resetKey]);

  return {
    selectedGap,
    gapSuggestion,
    gapSuggestOpen: selectedGap !== null,
    gapSuggestPending: suggestGapFillMutation.isPending,
    gapSuggestSaving: createGapFillMutation.isPending,
    gapSuggestError: suggestGapFillMutation.error,
    handleSelectGap,
    handleCloseGapSuggest,
    handleRetryGapSuggest,
    handleConfirmGapSuggest,
    resetGapSuggestState,
  };
}
