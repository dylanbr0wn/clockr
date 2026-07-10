import { useMemo, useState, type Dispatch, type SetStateAction } from "react";
import type { GapFill } from "@/lib/api";
import type { SchedulerCreateRequest } from "@/lib/scheduler";
import type {
  AllDayChip,
  ScheduleChange,
  ScheduleItem,
  SchedulePlacement,
} from "@/lib/schedule";
import {
  buildGapFillsByItemId,
  buildResettableDays,
  gapFillItemId,
  isEventItemId,
  isGapFillItemId,
  parseEventItemId,
} from "@/lib/schedule";
import type { ScheduleEventEditValues } from "./schedulePage.types";

interface Mutations {
  createManualEventMutation: {
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
  };
  updateManualEventMutation: {
    mutate: (
      payload: {
        id: number;
        periodId: number;
        day: string;
        startMinutes: number;
        endMinutes: number;
        categoryId?: number;
        note: string;
      },
      options?: { onSuccess?: () => void; onSettled?: () => void },
    ) => void;
  };
  deleteManualEventMutation: {
    mutate: (
      payload: { id: number; periodId: number },
      options?: { onSuccess?: () => void },
    ) => void;
  };
  excludeEventMutation: {
    mutate: (
      payload: { eventId: number; periodId: number },
      options?: { onSuccess?: () => void },
    ) => void;
  };
}

interface UseSchedulePageEditorParams extends Mutations {
  activePeriodId: number | undefined;
  gapFills: GapFill[];
}

function eventIdFromScheduleItemId(itemId: string): number | null {
  return parseEventItemId(itemId);
}

export function useSchedulePageEditor({
  activePeriodId,
  gapFills,
  createManualEventMutation,
  updateManualEventMutation,
  deleteManualEventMutation,
  excludeEventMutation,
}: UseSchedulePageEditorParams) {
  const [draftPlacements, setDraftPlacements] = useState<Record<string, SchedulePlacement>>({});
  const [preview, setPreview] = useState<ScheduleChange | null>(null);
  const [editingItemId, setEditingItemId] = useState<string | null>(null);
  const [pendingCreate, setPendingCreate] = useState<SchedulerCreateRequest | null>(null);
  const gapFillsByItemId = useMemo(
    () => buildGapFillsByItemId(gapFills),
    [gapFills],
  );

  const clearForPeriodChange = () => {
    setDraftPlacements({});
    setPreview(null);
    setEditingItemId(null);
    setPendingCreate(null);
  };

  const handleCreate = (request: SchedulerCreateRequest) => {
    if (!activePeriodId) {
      return;
    }
    setEditingItemId(null);
    setPendingCreate(request);
  };

  const handleCommit = (change: ScheduleChange) => {
    if (isGapFillItemId(change.itemId)) {
      const gapFill = gapFillsByItemId.get(change.itemId);
      if (gapFill) {
        setDraftPlacements((current) => ({
          ...current,
          [change.itemId]: {
            day: change.day,
            startMinutes: change.startMinutes,
            endMinutes: change.endMinutes,
          },
        }));
        updateManualEventMutation.mutate(
          {
            id: gapFill.id,
            periodId: gapFill.periodId,
            day: change.day,
            startMinutes: change.startMinutes,
            endMinutes: change.endMinutes,
            categoryId: gapFill.categoryId,
            note: gapFill.note ?? "",
          },
          {
            onSettled: () => {
              setDraftPlacements((current) => {
                const next = { ...current };
                delete next[change.itemId];
                return next;
              });
            },
          },
        );
      }
      setPreview(null);
      return;
    }

    if (isEventItemId(change.itemId)) {
      setPreview(null);
      return;
    }
    setPreview(null);
  };

  const handleOpenEventEditor = (item: ScheduleItem) => {
    if (!item.metadata?.mutable || !gapFillsByItemId.has(item.id)) {
      return;
    }
    setPendingCreate(null);
    setEditingItemId(item.id);
  };

  const handleDuplicateEvent = (item: ScheduleItem) => {
    const gapFill = gapFillsByItemId.get(item.id);
    if (!gapFill) {
      return;
    }
    createManualEventMutation.mutate({
      periodId: gapFill.periodId,
      day: item.day,
      startMinutes: item.startMinutes,
      endMinutes: item.endMinutes,
      categoryId: gapFill.categoryId,
      note: gapFill.note ?? item.metadata?.title ?? "",
    });
  };

  const handleRemoveEvent = (item: ScheduleItem) => {
    const gapFill = gapFillsByItemId.get(item.id);
    if (!gapFill) {
      return;
    }
    deleteManualEventMutation.mutate(
      { id: gapFill.id, periodId: gapFill.periodId },
      {
        onSuccess: () => {
          setEditingItemId((current) => (current === item.id ? null : current));
        },
      },
    );
  };

  const handleExcludeEvent = (item: ScheduleItem) => {
    if (item.metadata?.excludable === false || !activePeriodId) {
      return;
    }
    const eventId = eventIdFromScheduleItemId(item.id);
    if (eventId == null) {
      return;
    }
    excludeEventMutation.mutate({ eventId, periodId: activePeriodId });
  };

  const handleExcludeAllDayChip = (chip: AllDayChip) => {
    if (chip.excludable === false || !activePeriodId) {
      return;
    }
    excludeEventMutation.mutate({
      eventId: chip.eventId,
      periodId: activePeriodId,
    });
  };

  const handleResetDay = (day: string) => {
    if (!buildResettableDays(gapFills).has(day)) {
      return;
    }

    const manualGapFills = gapFills.filter(
      (gapFill) => gapFill.day === day && gapFill.source === "manual",
    );

    const deletedItemIds = new Set(manualGapFills.map((gapFill) => gapFillItemId(gapFill.id)));
    setEditingItemId((current) =>
      current && deletedItemIds.has(current) ? null : current,
    );

    manualGapFills.forEach((gapFill) => {
      deleteManualEventMutation.mutate({
        id: gapFill.id,
        periodId: gapFill.periodId,
      });
    });
  };

  const handleCloseEventEditor = () => {
    setEditingItemId(null);
    setPendingCreate(null);
  };

  const handleSaveEventEdit = (values: ScheduleEventEditValues) => {
    if (pendingCreate && activePeriodId) {
      createManualEventMutation.mutate(
        {
          periodId: activePeriodId,
          day: values.day,
          startMinutes: values.startMinutes,
          endMinutes: values.endMinutes,
          categoryId: values.categoryId,
          note: values.note,
        },
        { onSuccess: () => setPendingCreate(null) },
      );
      return;
    }

    if (!editingItemId) {
      return;
    }

    const gapFill = gapFillsByItemId.get(editingItemId);
    if (!gapFill) {
      return;
    }
    const itemId = editingItemId;

    setDraftPlacements((current) => ({
      ...current,
      [itemId]: {
        day: values.day,
        startMinutes: values.startMinutes,
        endMinutes: values.endMinutes,
      },
    }));

    updateManualEventMutation.mutate(
      {
        id: gapFill.id,
        periodId: gapFill.periodId,
        day: values.day,
        startMinutes: values.startMinutes,
        endMinutes: values.endMinutes,
        categoryId: values.categoryId,
        note: values.note,
      },
      {
        onSuccess: () => setEditingItemId(null),
        onSettled: () => {
          setDraftPlacements((current) => {
            const next = { ...current };
            delete next[itemId];
            return next;
          });
        },
      },
    );
  };

  const setPreviewValue = setPreview as Dispatch<SetStateAction<ScheduleChange | null>>;

  return {
    draftPlacements,
    preview,
    setPreview: setPreviewValue,
    editingItemId,
    pendingCreate,
    setEditingItemId,
    setPendingCreate,
    clearForPeriodChange,
    handleCreate,
    handleCommit,
    handleOpenEventEditor,
    handleDuplicateEvent,
    handleRemoveEvent,
    handleExcludeEvent,
    handleExcludeAllDayChip,
    handleResetDay,
    handleCloseEventEditor,
    handleSaveEventEdit,
  };
}
