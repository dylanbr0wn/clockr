# scheduler

Headless, unstyled scheduling primitives for React. You own every pixel; the
library owns time math, overlap layout, and pointer/keyboard interactions.

- **Headless** — no CSS, no DOM structure imposed. Prop getters wire behavior
  onto your own markup (same pattern as Downshift / TanStack).
- **Controlled** — items live in your state. The scheduler emits intents
  (`onCreate`, `onCommitChange`); you decide what to persist.
- **Typed metadata** — `SchedulerItem<TMetadata>` and `SchedulerDay<TMetadata>`
  carry your domain data through every callback and layout result.

## Quick start

```tsx
import { useScheduler } from "@/lib/scheduler";

function Calendar({ days, items, onChange }) {
  const scheduler = useScheduler({
    days,        // [{ date: "2026-06-08", label: "Mon" }, ...]
    items,       // [{ id, day: "2026-06-08", startMinutes: 540, endMinutes: 600 }, ...]
    onCreate: (request) => onChange.create(request),
    onCommitChange: (change) => onChange.update(change),
  });

  return (
    <div {...scheduler.getRootProps()}>
      {scheduler.days.map((day) => (
        <div key={day.date} {...scheduler.getDayColumnProps(day)}>
          {(scheduler.layoutsByDay[day.date] ?? []).map((layoutItem) => (
            <div key={layoutItem.item.id} {...scheduler.getItemProps(layoutItem)}>
              <div {...scheduler.getResizeHandleProps(layoutItem, "start")} />
              {/* your item content */}
              <div {...scheduler.getResizeHandleProps(layoutItem, "end")} />
            </div>
          ))}
        </div>
      ))}
    </div>
  );
}
```

Items are positioned with percentage-based `top/height/left/width` styles, so
day columns just need `position: relative` (applied by `getDayColumnProps`)
and a height.

## Interactions

| Gesture | Result |
| --- | --- |
| Drag an item | Move (across days too) — `onPreviewChange` per step, `onCommitChange` on release |
| Drag a resize handle | Adjust start or end, clamped to `minDurationMinutes` |
| Click empty column space | `onCreate` with `createDurationMinutes` at the clicked slot |
| Drag empty column space | `onCreate` with the dragged range (`dragToCreate`), ghost exposed via `createPreview` and injected into layouts as `CREATE_PREVIEW_ITEM_ID` |
| `↑` / `↓` on a focused item | Move by one slot |
| `←` / `→` | Move to the previous / next day |
| `Shift+↑` / `Shift+↓` | Shrink / grow the end |
| `Alt+↑` / `Alt+↓` | Grow / shrink from the start |

Pointer movements below `dragThresholdPx` stay clicks: no preview, no commit,
and item `onClick` handlers fire normally. Commits are skipped when nothing
actually changed.

## Config

All fields optional; defaults shown.

```ts
config: {
  slotMinutes: 15,             // snap grid
  minDurationMinutes: 15,      // smallest allowed item
  createDurationMinutes: 60,   // duration for click-to-create
  maxDays: 14,                 // hard cap on rendered days
  workingStartMinutes: 480,    // 08:00 — minimum visible range
  workingEndMinutes: 1080,     // 18:00 — expands to fit items
  dragThresholdPx: 4,          // pixels before a drag starts
  dragToCreate: true,          // drag empty space to create a range
  keyboard: true,              // arrow-key move/resize
}
```

The visible range is the working window expanded to fit all items, floored /
ceiled to the slot grid so nothing is clipped.

## Constraining changes

`transformChange` runs before every preview and commit. Return an adjusted
change to accept it, or `null` to reject and keep the previous state:

```ts
useScheduler({
  // forbid overlap with locked items
  transformChange: (change) =>
    lockedRanges.some((range) => intersects(range, change)) ? null : change,
});
```

## Components

Thin wrappers for the prop getters when you prefer JSX: `Scheduler`
(render-prop around `useScheduler`), `SchedulerRoot`, `SchedulerDayColumn`,
`SchedulerItemLayer`, `SchedulerTimeAxis`.

## Utilities

`formatMinutes`, `addDays` (timezone-safe), `snapMinutes` /
`snapMinutesDown` / `snapMinutesUp`, `minutesToPercent`, `packOverlaps`
(greedy lane packing for overlapping items), `MINUTES_PER_DAY`,
`DEFAULT_SCHEDULER_CONFIG`.
