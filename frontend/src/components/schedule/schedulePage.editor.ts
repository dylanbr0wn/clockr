import { useMemo, useState, type Dispatch, type SetStateAction } from "react";
import type { TimeEntry } from "@/lib/api";
import type { SchedulerCreateRequest } from "@/lib/scheduler";
import type {
  AllDayChip,
  ScheduleChange,
  ScheduleItem,
  SchedulePlacement,
} from "@/lib/schedule";
import {
  buildTimeEntriesByItemId,
  buildResettableDays,
  timeEntryItemId,
  isEventItemId,
  isTimeEntryItemId,
  parseEventItemId,
} from "@/lib/schedule";
import type { ScheduleEventEditValues } from "./schedulePage.types";

interface Mutations {
  createTimeEntryMutation: {
    mutate: (
      payload: {
        periodId: number;
        day: string;
        startMinutes: number;
        endMinutes: number;
        categoryId?: number;
        description: string;
        workType?: string;
        projectId?: number;
        billableStatus?: string;
      },
      options?: { onSuccess?: () => void },
    ) => void;
  };
  updateTimeEntryMutation: {
    mutate: (
      payload: {
        id: number;
        periodId: number;
        day: string;
        startMinutes: number;
        endMinutes: number;
        categoryId?: number;
        description: string;
        workType?: string;
        projectId?: number;
        billableStatus?: string;
      },
      options?: { onSuccess?: () => void; onSettled?: () => void },
    ) => void;
  };
  adjustDraftTimeEntryMutation: {
    mutate: (
      payload: {
        id: number;
        periodId: number;
        day: string;
        startMinutes: number;
        endMinutes: number;
        categoryId?: number;
        description: string;
        workType?: string;
        projectId?: number;
        billableStatus?: string;
      },
      options?: { onSuccess?: () => void; onSettled?: () => void },
    ) => void;
  };
  deleteTimeEntryMutation: {
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
  timeEntries: TimeEntry[];
}

function eventIdFromScheduleItemId(itemId: string): number | null {
  return parseEventItemId(itemId);
}

function editDescription(values: ScheduleEventEditValues): string {
  return values.description || values.note;
}

function allocationFields(values: {
  workType: string;
  projectId?: number;
  billableStatus: string;
}) {
  return {
    workType: values.workType,
    billableStatus: values.billableStatus,
    ...(typeof values.projectId === "number"
      ? { projectId: values.projectId }
      : {}),
  };
}

function timeEntryAllocation(timeEntry: TimeEntry) {
  return allocationFields({
    workType: timeEntry.workType,
    projectId: timeEntry.projectId,
    billableStatus: timeEntry.billableStatus,
  });
}

export function useSchedulePageEditor({
  activePeriodId,
  timeEntries,
  createTimeEntryMutation,
  updateTimeEntryMutation,
  adjustDraftTimeEntryMutation,
  deleteTimeEntryMutation,
  excludeEventMutation,
}: UseSchedulePageEditorParams) {
  const [draftPlacements, setDraftPlacements] = useState<Record<string, SchedulePlacement>>({});
  const [preview, setPreview] = useState<ScheduleChange | null>(null);
  const [editingItemId, setEditingItemId] = useState<string | null>(null);
  const [draftingItemId, setDraftingItemId] = useState<string | null>(null);
  const [pendingCreate, setPendingCreate] = useState<SchedulerCreateRequest | null>(null);
  const timeEntriesByItemId = useMemo(
    () => buildTimeEntriesByItemId(timeEntries),
    [timeEntries],
  );

  const clearForPeriodChange = () => {
    setDraftPlacements({});
    setPreview(null);
    setEditingItemId(null);
    setDraftingItemId(null);
    setPendingCreate(null);
  };

  const handleCreate = (request: SchedulerCreateRequest) => {
    if (!activePeriodId) {
      return;
    }
    setEditingItemId(null);
    setDraftingItemId(null);
    setPendingCreate(request);
  };

  const handleCommit = (change: ScheduleChange) => {
    if (isTimeEntryItemId(change.itemId)) {
      const timeEntry = timeEntriesByItemId.get(change.itemId);
      if (timeEntry) {
        setDraftPlacements((current) => ({
          ...current,
          [change.itemId]: {
            day: change.day,
            startMinutes: change.startMinutes,
            endMinutes: change.endMinutes,
          },
        }));
        const payload = {
          id: timeEntry.id,
          periodId: timeEntry.periodId,
          day: change.day,
          startMinutes: change.startMinutes,
          endMinutes: change.endMinutes,
          categoryId: timeEntry.categoryId,
          description: timeEntry.description ?? "",
          ...timeEntryAllocation(timeEntry),
        };
        const clearPlacement = {
          onSettled: () => {
            setDraftPlacements((current) => {
              const next = { ...current };
              delete next[change.itemId];
              return next;
            });
          },
        };
        if (timeEntry.attestation === "draft") {
          adjustDraftTimeEntryMutation.mutate(payload, clearPlacement);
        } else {
          updateTimeEntryMutation.mutate(payload, clearPlacement);
        }
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
    if (
      item.metadata?.opensDraftEditor ||
      !item.metadata?.mutable ||
      !timeEntriesByItemId.has(item.id)
    ) {
      return;
    }
    setPendingCreate(null);
    setDraftingItemId(null);
    setEditingItemId(item.id);
  };

  const handleOpenDraftEditor = (item: ScheduleItem) => {
    if (!item.metadata?.opensDraftEditor || !timeEntriesByItemId.has(item.id)) {
      return;
    }
    setPendingCreate(null);
    setEditingItemId(null);
    setDraftingItemId(item.id);
  };

  const handleDuplicateEvent = (item: ScheduleItem) => {
    const timeEntry = timeEntriesByItemId.get(item.id);
    if (!timeEntry || timeEntry.attestation === "draft") {
      return;
    }
    createTimeEntryMutation.mutate({
      periodId: timeEntry.periodId,
      day: item.day,
      startMinutes: item.startMinutes,
      endMinutes: item.endMinutes,
      categoryId: timeEntry.categoryId,
      description: timeEntry.description ?? item.metadata?.title ?? "",
      ...timeEntryAllocation(timeEntry),
    });
  };

  const handleRemoveEvent = (item: ScheduleItem) => {
    const timeEntry = timeEntriesByItemId.get(item.id);
    if (!timeEntry || timeEntry.attestation === "draft") {
      return;
    }
    deleteTimeEntryMutation.mutate(
      { id: timeEntry.id, periodId: timeEntry.periodId },
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
    if (!buildResettableDays(timeEntries).has(day)) {
      return;
    }

    const manualTimeEntries = timeEntries.filter(
      (timeEntry) =>
        timeEntry.localWorkDate === day &&
        !timeEntry.method &&
        timeEntry.attestation !== "draft",
    );

    const deletedItemIds = new Set(
      manualTimeEntries.map((timeEntry) => timeEntryItemId(timeEntry.id)),
    );
    setEditingItemId((current) =>
      current && deletedItemIds.has(current) ? null : current,
    );
    setDraftingItemId((current) =>
      current && deletedItemIds.has(current) ? null : current,
    );

    manualTimeEntries.forEach((timeEntry) => {
      deleteTimeEntryMutation.mutate({
        id: timeEntry.id,
        periodId: timeEntry.periodId,
      });
    });
  };

  const handleCloseEventEditor = () => {
    setEditingItemId(null);
    setPendingCreate(null);
  };

  const handleCloseDraftEditor = () => {
    setDraftingItemId(null);
  };

  const handleSaveEventEdit = (values: ScheduleEventEditValues) => {
    const description = editDescription(values);

    if (pendingCreate && activePeriodId) {
      createTimeEntryMutation.mutate(
        {
          periodId: activePeriodId,
          day: values.day,
          startMinutes: values.startMinutes,
          endMinutes: values.endMinutes,
          categoryId: values.categoryId,
          description,
          ...allocationFields(values),
        },
        { onSuccess: () => setPendingCreate(null) },
      );
      return;
    }

    if (!editingItemId) {
      return;
    }

    const timeEntry = timeEntriesByItemId.get(editingItemId);
    if (!timeEntry || timeEntry.attestation === "draft") {
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

    updateTimeEntryMutation.mutate(
      {
        id: timeEntry.id,
        periodId: timeEntry.periodId,
        day: values.day,
        startMinutes: values.startMinutes,
        endMinutes: values.endMinutes,
        categoryId: values.categoryId,
        description,
        ...allocationFields(values),
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

  const handleAdjustDraft = (values: ScheduleEventEditValues) => {
    if (!draftingItemId) {
      return;
    }
    const timeEntry = timeEntriesByItemId.get(draftingItemId);
    if (!timeEntry || timeEntry.attestation !== "draft") {
      return;
    }
    const itemId = draftingItemId;
    const description = editDescription(values);

    setDraftPlacements((current) => ({
      ...current,
      [itemId]: {
        day: values.day,
        startMinutes: values.startMinutes,
        endMinutes: values.endMinutes,
      },
    }));

    adjustDraftTimeEntryMutation.mutate(
      {
        id: timeEntry.id,
        periodId: timeEntry.periodId,
        day: values.day,
        startMinutes: values.startMinutes,
        endMinutes: values.endMinutes,
        categoryId: values.categoryId,
        description,
        ...allocationFields(values),
      },
      {
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
    draftingItemId,
    pendingCreate,
    setEditingItemId,
    setDraftingItemId,
    setPendingCreate,
    clearForPeriodChange,
    handleCreate,
    handleCommit,
    handleOpenEventEditor,
    handleOpenDraftEditor,
    handleDuplicateEvent,
    handleRemoveEvent,
    handleExcludeEvent,
    handleExcludeAllDayChip,
    handleResetDay,
    handleCloseEventEditor,
    handleCloseDraftEditor,
    handleSaveEventEdit,
    handleAdjustDraft,
  };
}
