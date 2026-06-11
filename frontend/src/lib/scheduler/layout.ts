import type {
  SchedulerItem,
  SchedulerLayoutItem,
  SchedulerVisibleRange,
} from "./types";
import { durationToPercent, minutesToPercent } from "./time";

interface LaneItem<TMetadata> {
  item: SchedulerItem<TMetadata>;
  lane: number;
}

function overlaps<TMetadata>(
  a: SchedulerItem<TMetadata>,
  b: SchedulerItem<TMetadata>,
) {
  return a.startMinutes < b.endMinutes && b.startMinutes < a.endMinutes;
}

function buildOverlapGroup<TMetadata>(items: SchedulerItem<TMetadata>[]) {
  const lanes: number[] = [];
  const assigned: LaneItem<TMetadata>[] = [];

  for (const item of items) {
    const lane = lanes.findIndex((laneEnd) => laneEnd <= item.startMinutes);
    const nextLane = lane === -1 ? lanes.length : lane;
    lanes[nextLane] = item.endMinutes;
    assigned.push({ item, lane: nextLane });
  }

  const laneCount = Math.max(1, lanes.length);
  return assigned.map(({ item, lane }) => ({
    item,
    lane,
    laneCount,
    overlaps: laneCount > 1,
  }));
}

export function packOverlaps<TMetadata>(
  items: SchedulerItem<TMetadata>[],
  visibleRange: SchedulerVisibleRange,
  previewItemId?: string,
): SchedulerLayoutItem<TMetadata>[] {
  const sorted = [...items].sort(
    (a, b) => a.startMinutes - b.startMinutes || a.endMinutes - b.endMinutes,
  );
  const groups: SchedulerItem<TMetadata>[][] = [];

  for (const item of sorted) {
    const group = groups.at(-1);
    const groupEnd = group
      ? Math.max(...group.map((groupItem) => groupItem.endMinutes))
      : -1;

    if (!group || item.startMinutes >= groupEnd) {
      groups.push([item]);
    } else {
      group.push(item);
    }
  }

  return groups.flatMap((group) =>
    buildOverlapGroup(group).map(({ item, lane, laneCount }) => {
      const widthPercent = 100 / laneCount;
      return {
        item,
        day: item.day,
        topPercent: minutesToPercent(item.startMinutes, visibleRange),
        heightPercent: durationToPercent(
          item.endMinutes - item.startMinutes,
          visibleRange,
        ),
        leftPercent: lane * widthPercent,
        widthPercent,
        lane,
        laneCount,
        overlaps: group.some(
          (candidate) => candidate.id !== item.id && overlaps(candidate, item),
        ),
        isPreview: item.id === previewItemId,
      };
    }),
  );
}
