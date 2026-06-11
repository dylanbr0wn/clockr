import type { HTMLAttributes, ReactNode } from "react";
import { useScheduler, type SchedulerApi } from "./useScheduler";
import { formatMinutes } from "./time";
import type {
  SchedulerDay,
  SchedulerLayoutItem,
  SchedulerOptions,
} from "./types";

export interface SchedulerProps<TItemMetadata = unknown, TDayMetadata = unknown>
  extends SchedulerOptions<TItemMetadata, TDayMetadata> {
  children: (scheduler: SchedulerApi<TItemMetadata, TDayMetadata>) => ReactNode;
}

export function Scheduler<TItemMetadata = unknown, TDayMetadata = unknown>({
  children,
  ...options
}: SchedulerProps<TItemMetadata, TDayMetadata>) {
  const scheduler = useScheduler(options);
  return <>{children(scheduler)}</>;
}

export interface SchedulerRootProps<TItemMetadata = unknown, TDayMetadata = unknown>
  extends HTMLAttributes<HTMLDivElement> {
  scheduler: SchedulerApi<TItemMetadata, TDayMetadata>;
}

export function SchedulerRoot<TItemMetadata = unknown, TDayMetadata = unknown>({
  scheduler,
  ...props
}: SchedulerRootProps<TItemMetadata, TDayMetadata>) {
  return <div {...scheduler.getRootProps(props)} />;
}

export interface SchedulerDayColumnProps<
  TItemMetadata = unknown,
  TDayMetadata = unknown,
> extends HTMLAttributes<HTMLDivElement> {
  scheduler: SchedulerApi<TItemMetadata, TDayMetadata>;
  day: SchedulerDay<TDayMetadata>;
}

export function SchedulerDayColumn<TItemMetadata = unknown, TDayMetadata = unknown>({
  scheduler,
  day,
  ...props
}: SchedulerDayColumnProps<TItemMetadata, TDayMetadata>) {
  return <div {...scheduler.getDayColumnProps(day, props)} />;
}

export interface SchedulerItemLayerProps<
  TItemMetadata = unknown,
  TDayMetadata = unknown,
> extends Omit<HTMLAttributes<HTMLDivElement>, "children"> {
  scheduler: SchedulerApi<TItemMetadata, TDayMetadata>;
  day: SchedulerDay<TDayMetadata>;
  children: (layoutItem: SchedulerLayoutItem<TItemMetadata>) => ReactNode;
}

export function SchedulerItemLayer<TItemMetadata = unknown, TDayMetadata = unknown>({
  scheduler,
  day,
  children,
  style,
  ...props
}: SchedulerItemLayerProps<TItemMetadata, TDayMetadata>) {
  const items = scheduler.layoutsByDay[day.date] ?? [];

  return (
    <div
      {...props}
      data-scheduler-layer={day.date}
      style={{
        position: "absolute",
        inset: 0,
        ...style,
      }}
    >
      {items.map((layoutItem) => children(layoutItem))}
    </div>
  );
}

export interface SchedulerTimeAxisProps<
  TItemMetadata = unknown,
  TDayMetadata = unknown,
> extends Omit<HTMLAttributes<HTMLDivElement>, "children"> {
  scheduler: SchedulerApi<TItemMetadata, TDayMetadata>;
  stepMinutes?: number;
  children?: (minutes: number, label: string) => ReactNode;
}

export function SchedulerTimeAxis<TItemMetadata = unknown, TDayMetadata = unknown>({
  scheduler,
  stepMinutes = 60,
  children,
  ...props
}: SchedulerTimeAxisProps<TItemMetadata, TDayMetadata>) {
  const step = Math.max(1, stepMinutes);
  const marks: number[] = [];
  for (
    let minute = scheduler.visibleRange.startMinutes;
    minute <= scheduler.visibleRange.endMinutes;
    minute += step
  ) {
    marks.push(minute);
  }

  return (
    <div {...props}>
      {marks.map((minute) => {
        const label = formatMinutes(minute);
        return (
          <div key={minute} data-scheduler-time={minute}>
            {children ? children(minute, label) : label}
          </div>
        );
      })}
    </div>
  );
}
