import { useEffect, type RefObject } from "react";
import {
  SCHEDULE_END_MINUTES,
  SCHEDULE_START_MINUTES,
} from "@/lib/schedule";
import {
  computeInitialTimelineScrollTop,
  computeTimelineHeight,
} from "./ScheduleTimeline.helpers";

export function useInitialTimelineScroll<T extends HTMLElement>(
  schedulerViewportRef: RefObject<T>,
) {
  useEffect(() => {
    const viewport = schedulerViewportRef.current;
    if (!viewport) {
      return;
    }

    const visibleRange = {
      startMinutes: SCHEDULE_START_MINUTES,
      endMinutes: SCHEDULE_END_MINUTES,
    };

    viewport.scrollTop = computeInitialTimelineScrollTop({
      visibleRange,
      timelineHeight: computeTimelineHeight(visibleRange),
    });
  }, [schedulerViewportRef]);
}
