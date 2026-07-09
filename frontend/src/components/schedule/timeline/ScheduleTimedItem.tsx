import {
  AlertTriangleIcon,
  CopyIcon,
  PencilIcon,
  Trash2Icon,
} from "lucide-react";
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from "@/components/ui/context-menu";
import { CREATE_PREVIEW_ITEM_ID } from "@/lib/scheduler";
import {
  durationLabel,
  formatTimeRange,
  scheduleItemPresentation,
  type ScheduleItem,
} from "@/lib/schedule";
import type { ScheduleTimelineActions } from "../ScheduleTimeline.types";
import type { TimelineLayoutItem, TimelineScheduler } from "./types";

interface ScheduleTimedItemProps {
  scheduler: TimelineScheduler;
  layoutItem: TimelineLayoutItem;
  actions: ScheduleTimelineActions;
}

export function ScheduleTimedItem({
  scheduler,
  layoutItem,
  actions,
}: ScheduleTimedItemProps) {
  const item = layoutItem.item;

  if (item.id === CREATE_PREVIEW_ITEM_ID) {
    return (
      <div
        {...scheduler.getItemProps(layoutItem, {
          className:
            "pointer-events-none select-none z-20 flex flex-col justify-center rounded-md border border-dashed border-amber-300 bg-amber-50/80 px-2 py-1 text-xs text-amber-950",
        })}
      >
        {formatTimeRange(item.startMinutes, item.endMinutes)}
      </div>
    );
  }

  const metadata = item.metadata;
  const presentation = metadata
    ? scheduleItemPresentation(metadata.kind, metadata.categoryColor)
    : { className: "border-border bg-muted text-foreground" };
  const canMutateItem = item.id.startsWith("gap-fill-");

  return (
    <ContextMenu>
      <ContextMenuTrigger asChild>
        <div
          {...scheduler.getItemProps(layoutItem, {
            onContextMenu: (event) => {
              event.stopPropagation();
            },
            onDoubleClick: (event) => {
              if (!canMutateItem) {
                return;
              }

              event.preventDefault();
              event.stopPropagation();
              actions.onEditItem(item as ScheduleItem);
            },
            className: [
              "group z-10 flex min-h-10 flex-col overflow-hidden rounded-md border px-2 py-1 text-left text-xs shadow-sm transition-shadow",
              canMutateItem
                ? "cursor-grab active:cursor-grabbing"
                : "cursor-default",
              layoutItem.isPreview
                ? "opacity-70 ring-2 ring-background/20"
                : "hover:shadow-md",
              presentation.className,
            ].join(" "),
            style: presentation.style,
          })}
        >
          {canMutateItem ? (
            <div
              {...scheduler.getResizeHandleProps(layoutItem, "start", {
                className:
                  "absolute inset-x-2 top-0 h-2 cursor-ns-resize rounded-full opacity-0 group-hover:opacity-100",
              })}
            />
          ) : null}
          <div className="min-w-0">
            {metadata?.kind === "review" ? (
              <div className="mb-1 flex items-center gap-1 text-[10px] font-semibold uppercase tracking-wide opacity-80">
                <AlertTriangleIcon className="size-3" />
                <span>Needs review</span>
              </div>
            ) : null}
            <p className="truncate font-semibold">
              {metadata?.title ?? "Untitled"}
            </p>
            <p className="truncate text-[11px] opacity-75">
              {formatTimeRange(item.startMinutes, item.endMinutes)} ·{" "}
              {durationLabel(item as ScheduleItem)}
            </p>
          </div>
          <div className="mt-auto truncate text-[11px] font-medium opacity-80">
            {metadata?.category ?? "Unassigned"}
          </div>
          {canMutateItem ? (
            <div
              {...scheduler.getResizeHandleProps(layoutItem, "end", {
                className:
                  "absolute inset-x-2 bottom-0 h-2 cursor-ns-resize rounded-full opacity-0 group-hover:opacity-100",
              })}
            />
          ) : null}
        </div>
      </ContextMenuTrigger>
      <ContextMenuContent data-scheduler-ignore-create="">
        <ContextMenuItem
          disabled={!canMutateItem}
          onSelect={() => actions.onEditItem(item as ScheduleItem)}
        >
          <PencilIcon />
          Edit
        </ContextMenuItem>
        <ContextMenuItem
          disabled={!canMutateItem}
          onSelect={() => actions.onDuplicateItem(item as ScheduleItem)}
        >
          <CopyIcon />
          Duplicate
        </ContextMenuItem>
        <ContextMenuSeparator />
        <ContextMenuItem
          disabled={!canMutateItem}
          variant="destructive"
          onSelect={() => actions.onRemoveItem(item as ScheduleItem)}
        >
          <Trash2Icon />
          Remove
        </ContextMenuItem>
      </ContextMenuContent>
    </ContextMenu>
  );
}
