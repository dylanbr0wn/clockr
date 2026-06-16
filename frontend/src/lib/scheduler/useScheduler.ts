import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type CSSProperties,
  type HTMLAttributes,
  type KeyboardEvent as ReactKeyboardEvent,
  type MutableRefObject,
  type PointerEvent as ReactPointerEvent,
  type Ref,
} from "react";
import { packOverlaps } from "./layout";
import {
  MINUTES_PER_DAY,
  calculateVisibleRange,
  clamp,
  normalizeConfig,
  snapMinutes,
} from "./time";
import {
  CREATE_PREVIEW_ITEM_ID,
  type SchedulerChange,
  type SchedulerConfig,
  type SchedulerCreateRequest,
  type SchedulerDay,
  type SchedulerInteraction,
  type SchedulerItem,
  type SchedulerLayoutItem,
  type SchedulerOptions,
  type SchedulerVisibleRange,
} from "./types";

interface ItemInteraction<TMetadata> {
  kind: "item";
  interaction: SchedulerInteraction;
  item: SchedulerItem<TMetadata>;
  pointerStartMinute: number;
  startClientX: number;
  startClientY: number;
  activated: boolean;
  lastChange: SchedulerChange<TMetadata>;
}

interface CreateInteraction {
  kind: "create";
  day: string;
  anchorMinute: number;
  startClientX: number;
  startClientY: number;
  activated: boolean;
  lastRange: SchedulerCreateRequest | null;
}

type ActiveInteraction<TMetadata> = ItemInteraction<TMetadata> | CreateInteraction;

type ElementProps<TElement extends HTMLElement> = HTMLAttributes<TElement> & {
  ref?: Ref<TElement>;
  [key: `data-${string}`]: string | boolean | undefined;
};

function assignRef<TElement>(ref: Ref<TElement> | undefined, value: TElement | null) {
  if (!ref) {
    return;
  }

  if (typeof ref === "function") {
    ref(value);
    return;
  }

  (ref as MutableRefObject<TElement | null>).current = value;
}

function composeHandlers<TEvent>(
  userHandler: ((event: TEvent) => void) | undefined,
  schedulerHandler: (event: TEvent) => void,
) {
  return (event: TEvent) => {
    userHandler?.(event);
    schedulerHandler(event);
  };
}

function toItemChange<TMetadata>(
  item: SchedulerItem<TMetadata>,
  interaction: SchedulerInteraction,
  day: string,
  startMinutes: number,
  endMinutes: number,
): SchedulerChange<TMetadata> {
  return {
    itemId: item.id,
    day,
    startMinutes,
    endMinutes,
    interaction,
    item,
  };
}

function sameRange(
  a: { day: string; startMinutes: number; endMinutes: number },
  b: { day: string; startMinutes: number; endMinutes: number },
) {
  return (
    a.day === b.day &&
    a.startMinutes === b.startMinutes &&
    a.endMinutes === b.endMinutes
  );
}

function isWithinThreshold(
  event: PointerEvent,
  startClientX: number,
  startClientY: number,
  thresholdPx: number,
) {
  return (
    Math.abs(event.clientX - startClientX) < thresholdPx &&
    Math.abs(event.clientY - startClientY) < thresholdPx
  );
}

function targetBlocksColumnCreate(target: EventTarget | null) {
  return (
    target instanceof Element &&
    target.closest("[data-scheduler-item], [data-scheduler-ignore-create]") !== null
  );
}

export interface SchedulerApi<TItemMetadata = unknown, TDayMetadata = unknown> {
  days: SchedulerDay<TDayMetadata>[];
  items: SchedulerItem<TItemMetadata>[];
  visibleRange: SchedulerVisibleRange;
  config: SchedulerConfig;
  previewChange: SchedulerChange<TItemMetadata> | null;
  createPreview: SchedulerCreateRequest | null;
  layoutsByDay: Record<string, SchedulerLayoutItem<TItemMetadata>[]>;
  getRootProps: <TElement extends HTMLElement>(
    props?: ElementProps<TElement>,
  ) => ElementProps<TElement>;
  getDayColumnProps: <TElement extends HTMLElement>(
    day: SchedulerDay<TDayMetadata>,
    props?: ElementProps<TElement>,
  ) => ElementProps<TElement>;
  getItemProps: <TElement extends HTMLElement>(
    layoutItem: SchedulerLayoutItem<TItemMetadata>,
    props?: ElementProps<TElement>,
  ) => ElementProps<TElement>;
  getResizeHandleProps: <TElement extends HTMLElement>(
    layoutItem: SchedulerLayoutItem<TItemMetadata>,
    edge: "start" | "end",
    props?: ElementProps<TElement>,
  ) => ElementProps<TElement>;
}

export function useScheduler<TItemMetadata = unknown, TDayMetadata = unknown>({
  days,
  items,
  config: configOverrides,
  onCreate,
  onPreviewChange,
  onCommitChange,
  transformChange,
}: SchedulerOptions<TItemMetadata, TDayMetadata>): SchedulerApi<
  TItemMetadata,
  TDayMetadata
> {
  const config = useMemo(
    () => normalizeConfig(configOverrides),
    // Depend on primitive fields so an inline config object doesn't invalidate
    // every memo (and resubscribe window listeners) on each render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [
      configOverrides?.slotMinutes,
      configOverrides?.minDurationMinutes,
      configOverrides?.createDurationMinutes,
      configOverrides?.maxDays,
      configOverrides?.scheduleStartMinutes,
      configOverrides?.scheduleEndMinutes,
      configOverrides?.workingStartMinutes,
      configOverrides?.workingEndMinutes,
      configOverrides?.dragThresholdPx,
      configOverrides?.dragToCreate,
      configOverrides?.keyboard,
    ],
  );
  const activeDays = useMemo(
    () => days.slice(0, config.maxDays),
    [config.maxDays, days],
  );
  const dayDates = useMemo(
    () => activeDays.map((day) => day.date),
    [activeDays],
  );
  const dayRefs = useRef(new Map<string, HTMLElement>());
  const activeRef = useRef<ActiveInteraction<TItemMetadata> | null>(null);
  const suppressColumnClickUntilRef = useRef(0);
  const [previewChange, setPreviewChange] =
    useState<SchedulerChange<TItemMetadata> | null>(null);
  const [createPreview, setCreatePreview] =
    useState<SchedulerCreateRequest | null>(null);

  const visibleRange = useMemo(
    () => calculateVisibleRange(items, config),
    [config, items],
  );

  const previewItems = useMemo(() => {
    if (!previewChange) {
      return items;
    }

    return items.map((item) =>
      item.id === previewChange.itemId
        ? {
            ...item,
            day: previewChange.day,
            startMinutes: previewChange.startMinutes,
            endMinutes: previewChange.endMinutes,
          }
        : item,
    );
  }, [items, previewChange]);

  const layoutsByDay = useMemo(() => {
    const layoutItems = createPreview
      ? [
          ...previewItems,
          {
            id: CREATE_PREVIEW_ITEM_ID,
            day: createPreview.day,
            startMinutes: createPreview.startMinutes,
            endMinutes: createPreview.endMinutes,
          } as SchedulerItem<TItemMetadata>,
        ]
      : previewItems;
    const previewId = previewChange?.itemId ?? (createPreview ? CREATE_PREVIEW_ITEM_ID : undefined);

    return activeDays.reduce<Record<string, SchedulerLayoutItem<TItemMetadata>[]>>(
      (layouts, day) => {
        layouts[day.date] = packOverlaps(
          layoutItems.filter((item) => item.day === day.date),
          visibleRange,
          previewId,
        );
        return layouts;
      },
      {},
    );
  }, [activeDays, createPreview, previewChange?.itemId, previewItems, visibleRange]);

  const pointToMinutes = useCallback(
    (clientY: number, day: string) => {
      const column = dayRefs.current.get(day);
      if (!column) {
        return visibleRange.startMinutes;
      }

      const rect = column.getBoundingClientRect();
      const percent = rect.height <= 0 ? 0 : clamp((clientY - rect.top) / rect.height, 0, 1);
      const rawMinutes =
        visibleRange.startMinutes +
        percent * (visibleRange.endMinutes - visibleRange.startMinutes);
      return snapMinutes(rawMinutes, config.slotMinutes);
    },
    [config.slotMinutes, visibleRange],
  );

  const dayAtX = useCallback(
    (clientX: number) => {
      for (const day of dayDates) {
        const column = dayRefs.current.get(day);
        if (!column) {
          continue;
        }
        const rect = column.getBoundingClientRect();
        if (clientX >= rect.left && clientX <= rect.right) {
          return day;
        }
      }
      return null;
    },
    [dayDates],
  );

  const calculateChange = useCallback(
    (event: PointerEvent, active: ItemInteraction<TItemMetadata>) => {
      const item = active.item;
      const duration = item.endMinutes - item.startMinutes;
      const targetDay =
        active.interaction === "move"
          ? dayAtX(event.clientX) ?? item.day
          : item.day;

      if (active.interaction === "move") {
        const pointerMinute = pointToMinutes(event.clientY, targetDay);
        const delta = pointerMinute - active.pointerStartMinute;
        const startMinutes = clamp(
          snapMinutes(item.startMinutes + delta, config.slotMinutes),
          0,
          MINUTES_PER_DAY - duration,
        );

        return toItemChange(
          item,
          active.interaction,
          targetDay,
          startMinutes,
          startMinutes + duration,
        );
      }

      const pointerMinute = pointToMinutes(event.clientY, item.day);

      if (active.interaction === "resize-start") {
        const startMinutes = clamp(
          pointerMinute,
          0,
          item.endMinutes - config.minDurationMinutes,
        );
        return toItemChange(
          item,
          active.interaction,
          item.day,
          startMinutes,
          item.endMinutes,
        );
      }

      const endMinutes = clamp(
        pointerMinute,
        item.startMinutes + config.minDurationMinutes,
        MINUTES_PER_DAY,
      );
      return toItemChange(
        item,
        active.interaction,
        item.day,
        item.startMinutes,
        endMinutes,
      );
    },
    [config.minDurationMinutes, config.slotMinutes, dayAtX, pointToMinutes],
  );

  const calculateCreateRange = useCallback(
    (event: PointerEvent, active: CreateInteraction): SchedulerCreateRequest => {
      const pointerMinute = pointToMinutes(event.clientY, active.day);
      let startMinutes = Math.min(active.anchorMinute, pointerMinute);
      let endMinutes = Math.max(active.anchorMinute, pointerMinute);

      if (endMinutes - startMinutes < config.minDurationMinutes) {
        endMinutes = startMinutes + config.minDurationMinutes;
        if (endMinutes > MINUTES_PER_DAY) {
          endMinutes = MINUTES_PER_DAY;
          startMinutes = endMinutes - config.minDurationMinutes;
        }
      }

      return { day: active.day, startMinutes, endMinutes };
    },
    [config.minDurationMinutes, pointToMinutes],
  );

  const suppressColumnClick = useCallback(() => {
    suppressColumnClickUntilRef.current = window.performance.now() + 350;
  }, []);

  const applyTransform = useCallback(
    (change: SchedulerChange<TItemMetadata>) =>
      transformChange ? transformChange(change) : change,
    [transformChange],
  );

  useEffect(() => {
    const handlePointerMove = (event: PointerEvent) => {
      const active = activeRef.current;
      if (!active) {
        return;
      }

      if (
        !active.activated &&
        isWithinThreshold(event, active.startClientX, active.startClientY, config.dragThresholdPx)
      ) {
        return;
      }
      active.activated = true;

      if (active.kind === "create") {
        const range = calculateCreateRange(event, active);
        if (active.lastRange && sameRange(range, active.lastRange)) {
          return;
        }
        active.lastRange = range;
        setCreatePreview(range);
        return;
      }

      const change = applyTransform(calculateChange(event, active));
      if (!change || sameRange(change, active.lastChange)) {
        return;
      }
      active.lastChange = change;
      setPreviewChange(change);
      onPreviewChange?.(change);
    };

    const handlePointerUp = () => {
      const active = activeRef.current;
      if (!active) {
        return;
      }

      activeRef.current = null;

      if (active.kind === "create") {
        setCreatePreview(null);
        if (active.activated && active.lastRange) {
          suppressColumnClick();
          onCreate?.(active.lastRange);
        }
        return;
      }

      setPreviewChange(null);
      if (active.activated) {
        suppressColumnClick();
        if (!sameRange(active.lastChange, active.item)) {
          onCommitChange?.(active.lastChange);
        }
      }
    };

    window.addEventListener("pointermove", handlePointerMove);
    window.addEventListener("pointerup", handlePointerUp);
    window.addEventListener("pointercancel", handlePointerUp);

    return () => {
      window.removeEventListener("pointermove", handlePointerMove);
      window.removeEventListener("pointerup", handlePointerUp);
      window.removeEventListener("pointercancel", handlePointerUp);
    };
  }, [
    applyTransform,
    calculateChange,
    calculateCreateRange,
    config.dragThresholdPx,
    onCommitChange,
    onCreate,
    onPreviewChange,
    suppressColumnClick,
  ]);

  const startInteraction = useCallback(
    (
      interaction: SchedulerInteraction,
      item: SchedulerItem<TItemMetadata>,
      event: ReactPointerEvent<HTMLElement>,
    ) => {
      if (item.disabled || event.button !== 0) {
        return;
      }

      event.preventDefault();
      event.stopPropagation();

      activeRef.current = {
        kind: "item",
        interaction,
        item,
        pointerStartMinute: pointToMinutes(event.clientY, item.day),
        startClientX: event.clientX,
        startClientY: event.clientY,
        activated: false,
        lastChange: toItemChange(
          item,
          interaction,
          item.day,
          item.startMinutes,
          item.endMinutes,
        ),
      };
    },
    [pointToMinutes],
  );

  const commitKeyboardChange = useCallback(
    (change: SchedulerChange<TItemMetadata>) => {
      const final = applyTransform(change);
      if (!final || sameRange(final, change.item)) {
        return;
      }
      onCommitChange?.(final);
    },
    [applyTransform, onCommitChange],
  );

  const handleItemKeyDown = useCallback(
    (item: SchedulerItem<TItemMetadata>, event: ReactKeyboardEvent<HTMLElement>) => {
      if (!config.keyboard || event.defaultPrevented || item.disabled) {
        return;
      }

      const slot = config.slotMinutes;
      const duration = item.endMinutes - item.startMinutes;
      let change: SchedulerChange<TItemMetadata> | null = null;

      switch (event.key) {
        case "ArrowUp":
        case "ArrowDown": {
          const direction = event.key === "ArrowUp" ? -1 : 1;
          if (event.shiftKey) {
            const endMinutes = clamp(
              item.endMinutes + direction * slot,
              item.startMinutes + config.minDurationMinutes,
              MINUTES_PER_DAY,
            );
            change = toItemChange(item, "resize-end", item.day, item.startMinutes, endMinutes);
          } else if (event.altKey) {
            const startMinutes = clamp(
              item.startMinutes + direction * slot,
              0,
              item.endMinutes - config.minDurationMinutes,
            );
            change = toItemChange(item, "resize-start", item.day, startMinutes, item.endMinutes);
          } else {
            const startMinutes = clamp(
              item.startMinutes + direction * slot,
              0,
              MINUTES_PER_DAY - duration,
            );
            change = toItemChange(
              item,
              "move",
              item.day,
              startMinutes,
              startMinutes + duration,
            );
          }
          break;
        }
        case "ArrowLeft":
        case "ArrowRight": {
          const offset = event.key === "ArrowLeft" ? -1 : 1;
          const dayIndex = dayDates.indexOf(item.day);
          const targetDay = dayDates[dayIndex + offset];
          if (dayIndex === -1 || !targetDay) {
            return;
          }
          change = toItemChange(item, "move", targetDay, item.startMinutes, item.endMinutes);
          break;
        }
        default:
          return;
      }

      event.preventDefault();
      commitKeyboardChange(change);
    },
    [commitKeyboardChange, config.keyboard, config.minDurationMinutes, config.slotMinutes, dayDates],
  );

  const getRootProps = useCallback(
    <TElement extends HTMLElement>(
      props: ElementProps<TElement> = {},
    ): ElementProps<TElement> => ({
      ...props,
      role: props.role ?? "application",
      "aria-label": props["aria-label"] ?? "Scheduler",
      "data-scheduler-root": "",
    }),
    [],
  );

  const getDayColumnProps = useCallback(
    <TElement extends HTMLElement>(
      day: SchedulerDay<TDayMetadata>,
      props: ElementProps<TElement> = {},
    ): ElementProps<TElement> => {
      const handlePointerDown = (event: ReactPointerEvent<TElement>) => {
        if (
          event.defaultPrevented ||
          event.button !== 0 ||
          day.disabled ||
          !onCreate ||
          !config.dragToCreate ||
          targetBlocksColumnCreate(event.target)
        ) {
          return;
        }

        activeRef.current = {
          kind: "create",
          day: day.date,
          anchorMinute: pointToMinutes(event.clientY, day.date),
          startClientX: event.clientX,
          startClientY: event.clientY,
          activated: false,
          lastRange: null,
        };
      };

      const handleClick = (event: React.MouseEvent<TElement>) => {
        if (
          event.defaultPrevented ||
          day.disabled ||
          !onCreate ||
          targetBlocksColumnCreate(event.target)
        ) {
          return;
        }

        if (window.performance.now() < suppressColumnClickUntilRef.current) {
          event.preventDefault();
          event.stopPropagation();
          return;
        }

        const startMinutes = clamp(
          pointToMinutes(event.clientY, day.date),
          0,
          MINUTES_PER_DAY - config.createDurationMinutes,
        );
        onCreate({
          day: day.date,
          startMinutes,
          endMinutes: startMinutes + config.createDurationMinutes,
        });
      };

      const style: CSSProperties = {
        position: "relative",
        touchAction: "none",
        ...props.style,
      };

      return {
        ...props,
        ref: (node: TElement | null) => {
          if (node) {
            dayRefs.current.set(day.date, node);
          } else {
            dayRefs.current.delete(day.date);
          }
          assignRef(props.ref, node);
        },
        role: props.role ?? "gridcell",
        "aria-disabled": day.disabled || undefined,
        "data-scheduler-day": day.date,
        style,
        onPointerDown: composeHandlers(props.onPointerDown, handlePointerDown),
        onClick: composeHandlers(props.onClick, handleClick),
      };
    },
    [config.createDurationMinutes, config.dragToCreate, onCreate, pointToMinutes],
  );

  const getItemProps = useCallback(
    <TElement extends HTMLElement>(
      layoutItem: SchedulerLayoutItem<TItemMetadata>,
      props: ElementProps<TElement> = {},
    ): ElementProps<TElement> => {
      const item = layoutItem.item;
      const style: CSSProperties = {
        position: "absolute",
        top: `${layoutItem.topPercent}%`,
        height: `${layoutItem.heightPercent}%`,
        left: `${layoutItem.leftPercent}%`,
        width: `${layoutItem.widthPercent}%`,
        touchAction: "none",
        ...props.style,
      };

      return {
        ...props,
        role: props.role ?? "button",
        tabIndex: props.tabIndex ?? (item.disabled ? -1 : 0),
        "aria-disabled": item.disabled || undefined,
        "data-scheduler-item": item.id,
        "data-scheduler-preview": layoutItem.isPreview || undefined,
        style,
        onPointerDown: composeHandlers(props.onPointerDown, (event) =>
          startInteraction("move", item, event),
        ),
        onKeyDown: composeHandlers(props.onKeyDown, (event) =>
          handleItemKeyDown(item, event),
        ),
      };
    },
    [handleItemKeyDown, startInteraction],
  );

  const getResizeHandleProps = useCallback(
    <TElement extends HTMLElement>(
      layoutItem: SchedulerLayoutItem<TItemMetadata>,
      edge: "start" | "end",
      props: ElementProps<TElement> = {},
    ): ElementProps<TElement> => {
      const item = layoutItem.item;

      return {
        ...props,
        role: props.role ?? "separator",
        "aria-label":
          props["aria-label"] ??
          (edge === "start" ? "Resize start time" : "Resize end time"),
        "data-scheduler-resize": edge,
        onPointerDown: composeHandlers(props.onPointerDown, (event) =>
          startInteraction(edge === "start" ? "resize-start" : "resize-end", item, event),
        ),
      };
    },
    [startInteraction],
  );

  return {
    days: activeDays,
    items: previewItems,
    visibleRange,
    config,
    previewChange,
    createPreview,
    layoutsByDay,
    getRootProps,
    getDayColumnProps,
    getItemProps,
    getResizeHandleProps,
  };
}
