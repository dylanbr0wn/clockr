import {
  useEffect,
  useMemo,
  useState,
  type Dispatch,
  type SetStateAction,
} from "react";
import {
  useCategories,
  useCreateManualEvent,
  useCurrentPeriod,
  useDeleteManualEvent,
  useEvents,
  useGapFills,
  useOpenReviewItems,
  usePeriods,
  useTzSegments,
  useUpdateManualEvent,
  type Category,
  type Period,
} from "@/lib/api";
import type { SchedulerCreateRequest } from "@/lib/scheduler";
import {
  START_DATE,
  buildDays,
  defaultTimeZone,
  eventToSchedulerItem,
  gapFillToSchedulerItem,
  localDateKey,
  periodContainsDate,
  periodDayCount,
  type ScheduleChange,
  type ScheduleDay,
  type ScheduleItem,
  type ScheduleMetadata,
  type SchedulePlacement,
} from "@/lib/schedule";

export type ScheduleViewDayCount = 1 | 7 | 14;

export const SCHEDULE_VIEW_DAY_OPTIONS: ScheduleViewDayCount[] = [1, 7, 14];

export interface SchedulePageViewModel {
  selectedPeriodId: number | null;
  setSelectedPeriodId: Dispatch<SetStateAction<number | null>>;
  viewDayCount: ScheduleViewDayCount;
  setViewDayCount: Dispatch<SetStateAction<ScheduleViewDayCount>>;
  periods: Period[];
  activePeriod: Period | null;
  activePeriodId: number | undefined;
  categories: Category[];
  days: ScheduleDay[];
  items: ScheduleItem[];
  resettableDays: ReadonlySet<string>;
  totals: Record<string, number>;
  visibleDayCount: number;
  preview: ScheduleChange | null;
  setPreview: Dispatch<SetStateAction<ScheduleChange | null>>;
  isBackendLoading: boolean;
  backendError: unknown;
  counts: {
    events: number;
    gapFills: number;
    categories: number;
    reviewItems: number;
  };
  createPending: boolean;
  editingEvent: EditableScheduleEvent | null;
  editEventPending: boolean;
  handleCreate: (request: SchedulerCreateRequest) => void;
  handleCommit: (change: ScheduleChange) => void;
  handleOpenEventEditor: (item: ScheduleItem) => void;
  handleDuplicateEvent: (item: ScheduleItem) => void;
  handleRemoveEvent: (item: ScheduleItem) => void;
  handleResetDay: (day: string) => void;
  handleCloseEventEditor: () => void;
  handleSaveEventEdit: (values: ScheduleEventEditValues) => void;
}

export interface EditableScheduleEvent {
  id: string;
  gapFillId: number;
  periodId: number;
  day: string;
  startMinutes: number;
  endMinutes: number;
  category: string;
  categoryId?: number;
  note: string;
}

export interface ScheduleEventEditValues {
  day: string;
  startMinutes: number;
  endMinutes: number;
  categoryId?: number;
  note: string;
}

export function useSchedulePage(): SchedulePageViewModel {
  const [selectedPeriodId, setSelectedPeriodId] = useState<number | null>(null);
  const [viewDayCount, setViewDayCount] =
    useState<ScheduleViewDayCount>(7);
  const [draftPlacements, setDraftPlacements] = useState<
    Record<string, SchedulePlacement>
  >({});
  const [preview, setPreview] = useState<ScheduleChange | null>(null);
  const [editingItemId, setEditingItemId] = useState<string | null>(null);
  const today = useMemo(() => localDateKey(), []);
  const currentTimeZone = useMemo(() => defaultTimeZone(), []);
  const periodsQuery = usePeriods();
  const currentPeriodQuery = useCurrentPeriod(today, currentTimeZone);
  const categoriesQuery = useCategories();
  const createManualEventMutation = useCreateManualEvent();
  const updateManualEventMutation = useUpdateManualEvent();
  const deleteManualEventMutation = useDeleteManualEvent();
  const persistedPeriods = useMemo(
    () => periodsQuery.data ?? [],
    [periodsQuery.data],
  );
  const currentPeriod = currentPeriodQuery.data ?? null;
  const periods = useMemo(() => {
    if (
      currentPeriod &&
      !persistedPeriods.some((period) => period.id === currentPeriod.id)
    ) {
      return [currentPeriod, ...persistedPeriods];
    }

    return persistedPeriods;
  }, [currentPeriod, persistedPeriods]);
  const categories = useMemo(
    () => categoriesQuery.data ?? [],
    [categoriesQuery.data],
  );
  const activePeriod = useMemo(
    () =>
      periods.find((period) => period.id === selectedPeriodId) ??
      currentPeriod ??
      periods.find((period) => periodContainsDate(period, today)) ??
      periods[0] ??
      null,
    [currentPeriod, periods, selectedPeriodId, today],
  );
  const activePeriodId = activePeriod?.id;
  const eventsQuery = useEvents(activePeriodId);
  const gapFillsQuery = useGapFills(activePeriodId);
  const reviewItemsQuery = useOpenReviewItems(activePeriodId);
  const tzSegmentsQuery = useTzSegments(activePeriodId);
  const categoriesById = useMemo(() => {
    return new Map(categories.map((category) => [category.id, category]));
  }, [categories]);
  const periodVisibleDayCount = activePeriod
    ? periodDayCount(activePeriod)
    : viewDayCount;
  const visibleDayCount = Math.min(periodVisibleDayCount, viewDayCount);
  const days = useMemo(
    () => buildDays(activePeriod?.startDate ?? START_DATE, visibleDayCount),
    [activePeriod?.startDate, visibleDayCount],
  );
  const gapFillsByItemId = useMemo(() => {
    return new Map(
      (gapFillsQuery.data ?? []).map((gapFill) => [
        `gap-fill-${gapFill.id}`,
        gapFill,
      ]),
    );
  }, [gapFillsQuery.data]);
  const resettableDays = useMemo(() => {
    return new Set(
      (gapFillsQuery.data ?? [])
        .filter((gapFill) => gapFill.source === "manual")
        .map((gapFill) => gapFill.day),
    );
  }, [gapFillsQuery.data]);
  const backendItems = useMemo(() => {
    const events = eventsQuery.data ?? [];
    const gapFills = gapFillsQuery.data ?? [];
    const tzSegments = tzSegmentsQuery.data ?? [];

    return [
      ...events
        .map((event) =>
          eventToSchedulerItem(
            event,
            tzSegments,
            draftPlacements[`event-${event.id}`],
          ),
        )
        .filter((item): item is ScheduleItem => item !== null),
      ...gapFills
        .map((gapFill) =>
          gapFillToSchedulerItem(
            gapFill,
            categoriesById,
            tzSegments,
            draftPlacements[`gap-fill-${gapFill.id}`],
          ),
        )
        .filter((item): item is ScheduleItem => item !== null),
    ];
  }, [
    categoriesById,
    draftPlacements,
    eventsQuery.data,
    gapFillsQuery.data,
    tzSegmentsQuery.data,
  ]);
  const items = backendItems;
  const editingEvent = useMemo(() => {
    if (!editingItemId) {
      return null;
    }

    const gapFill = gapFillsByItemId.get(editingItemId);
    const item = items.find((candidate) => candidate.id === editingItemId);

    if (!gapFill || !item) {
      return null;
    }

    return {
      id: editingItemId,
      gapFillId: gapFill.id,
      periodId: gapFill.periodId,
      day: item.day,
      startMinutes: item.startMinutes,
      endMinutes: item.endMinutes,
      category: item.metadata?.category ?? "Unassigned",
      categoryId: gapFill.categoryId,
      note: gapFill.note ?? "",
    };
  }, [editingItemId, gapFillsByItemId, items]);
  const totals = useMemo(() => {
    return items.reduce<Record<string, number>>((next, item) => {
      const key = item.metadata?.category ?? "Unassigned";
      next[key] = (next[key] ?? 0) + item.endMinutes - item.startMinutes;
      return next;
    }, {});
  }, [items]);
  const isBackendLoading =
    periodsQuery.isLoading ||
    currentPeriodQuery.isLoading ||
    categoriesQuery.isLoading ||
    eventsQuery.isLoading ||
    gapFillsQuery.isLoading ||
    reviewItemsQuery.isLoading ||
    tzSegmentsQuery.isLoading ||
    createManualEventMutation.isPending ||
    updateManualEventMutation.isPending ||
    deleteManualEventMutation.isPending;
  const backendError =
    periodsQuery.error ??
    currentPeriodQuery.error ??
    categoriesQuery.error ??
    eventsQuery.error ??
    gapFillsQuery.error ??
    reviewItemsQuery.error ??
    tzSegmentsQuery.error ??
    createManualEventMutation.error ??
    updateManualEventMutation.error ??
    deleteManualEventMutation.error;

  useEffect(() => {
    setSelectedPeriodId((current) => {
      if (
        currentPeriod &&
        (!current || !periods.some((period) => period.id === current))
      ) {
        return currentPeriod.id;
      }

      if (current && periods.some((period) => period.id === current)) {
        return current;
      }

      return currentPeriod?.id ?? periods[0]?.id ?? null;
    });
  }, [currentPeriod, periods]);

  useEffect(() => {
    setDraftPlacements({});
    setPreview(null);
    setEditingItemId(null);
  }, [activePeriodId]);

  const handleCreate = (request: SchedulerCreateRequest) => {
    if (activePeriodId) {
      createManualEventMutation.mutate({
        periodId: activePeriodId,
        day: request.day,
        startMinutes: request.startMinutes,
        endMinutes: request.endMinutes,
        note: "New block",
      });
      return;
    }
  };

  const handleCommit = (change: ScheduleChange) => {
    if (change.itemId.startsWith("gap-fill-")) {
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

    if (change.itemId.startsWith("event-")) {
      setDraftPlacements((current) => ({
        ...current,
        [change.itemId]: {
          day: change.day,
          startMinutes: change.startMinutes,
          endMinutes: change.endMinutes,
        },
      }));
    }

    setPreview(null);
  };

  const handleOpenEventEditor = (item: ScheduleItem) => {
    if (!item.id.startsWith("gap-fill-") || !gapFillsByItemId.has(item.id)) {
      return;
    }

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
      {
        id: gapFill.id,
        periodId: gapFill.periodId,
      },
      {
        onSuccess: () => {
          setEditingItemId((current) => (current === item.id ? null : current));
        },
      },
    );
  };

  const handleResetDay = (day: string) => {
    const manualGapFills = (gapFillsQuery.data ?? []).filter(
      (gapFill) => gapFill.day === day && gapFill.source === "manual",
    );

    if (manualGapFills.length === 0) {
      return;
    }

    const deletedItemIds = new Set(
      manualGapFills.map((gapFill) => `gap-fill-${gapFill.id}`),
    );

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
  };

  const handleSaveEventEdit = (values: ScheduleEventEditValues) => {
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
        onSuccess: () => {
          setEditingItemId(null);
        },
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

  return {
    selectedPeriodId,
    setSelectedPeriodId,
    viewDayCount,
    setViewDayCount,
    periods,
    activePeriod,
    activePeriodId,
    categories,
    days,
    items,
    resettableDays,
    totals,
    visibleDayCount,
    preview,
    setPreview: setPreview as Dispatch<
      SetStateAction<ScheduleChange | null>
    >,
    isBackendLoading,
    backendError,
    counts: {
      events: eventsQuery.data?.length ?? 0,
      gapFills: gapFillsQuery.data?.length ?? 0,
      categories: categories.length,
      reviewItems: reviewItemsQuery.data?.length ?? 0,
    },
    createPending: createManualEventMutation.isPending,
    editingEvent,
    editEventPending: updateManualEventMutation.isPending,
    handleCreate,
    handleCommit,
    handleOpenEventEditor,
    handleDuplicateEvent,
    handleRemoveEvent,
    handleResetDay,
    handleCloseEventEditor,
    handleSaveEventEdit,
  };
}
