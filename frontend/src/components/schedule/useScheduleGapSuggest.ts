import { useEffect, useState } from "react";
import type { GapEvidenceItem, GapSuggestion } from "@/lib/api";
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

interface ListGapEvidenceMutation {
  isPending: boolean;
  error: unknown;
  mutate: (
    payload: { start: string; end: string },
    options?: { onSuccess?: (items: GapEvidenceItem[]) => void },
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
      description: string;
    },
    options?: { onSuccess?: () => void },
  ) => void;
}

interface UseScheduleGapSuggestParams {
  activePeriodId: number | undefined;
  aiConfigured: boolean;
  suggestGapFillMutation: SuggestGapFillMutation;
  listGapEvidenceMutation: ListGapEvidenceMutation;
  createGapFillMutation: CreateGapFillMutation;
  resetKey: number | undefined;
}

export function useScheduleGapSuggest({
  activePeriodId,
  aiConfigured,
  suggestGapFillMutation,
  listGapEvidenceMutation,
  createGapFillMutation,
  resetKey,
}: UseScheduleGapSuggestParams) {
  const [selectedGap, setSelectedGap] = useState<SelectedGap | null>(null);
  const [gapSuggestion, setGapSuggestion] = useState<GapSuggestion | null>(null);
  const [gapEvidenceItems, setGapEvidenceItems] = useState<GapEvidenceItem[]>(
    [],
  );

  const requestGapSuggestion = (gap: SelectedGap) => {
    const window = {
      start: gap.gapWindowStart,
      end: gap.gapWindowEnd,
    };

    setGapSuggestion(null);
    setGapEvidenceItems([]);

    listGapEvidenceMutation.mutate(window, {
      onSuccess: (items) => {
        setGapEvidenceItems(items);
      },
    });

    suggestGapFillMutation.mutate(window, {
      onSuccess: (suggestion) => {
        setGapSuggestion(suggestion);
      },
    });
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
    setGapEvidenceItems([]);

    if (aiConfigured) {
      requestGapSuggestion(gap);
    }
  };

  const handleCloseGapSuggest = () => {
    setSelectedGap(null);
    setGapSuggestion(null);
    setGapEvidenceItems([]);
    suggestGapFillMutation.reset();
    listGapEvidenceMutation.reset();
  };

  const handleRetryGapSuggest = () => {
    if (!selectedGap) {
      return;
    }
    requestGapSuggestion(selectedGap);
  };

  const handleConfirmGapSuggest = (values: {
    categoryId?: number;
    description: string;
  }) => {
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
        description: values.description,
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
    setGapEvidenceItems([]);
  };

  useEffect(() => {
    resetGapSuggestState();
  }, [resetKey]);

  return {
    selectedGap,
    gapSuggestion,
    gapEvidenceItems,
    gapSuggestOpen: selectedGap !== null,
    gapSuggestPending: suggestGapFillMutation.isPending,
    gapEvidencePending: listGapEvidenceMutation.isPending,
    gapSuggestSaving: createGapFillMutation.isPending,
    gapSuggestError: suggestGapFillMutation.error,
    gapEvidenceError: listGapEvidenceMutation.error,
    handleSelectGap,
    handleCloseGapSuggest,
    handleRetryGapSuggest,
    handleConfirmGapSuggest,
    resetGapSuggestState,
  };
}
