import { Clock, Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { formatPeriodLabel } from "@/lib/schedule";
import {
  SCHEDULE_VIEW_DAY_OPTIONS,
  type SchedulePageViewModel,
  type ScheduleViewDayCount,
} from "./useSchedulePage";

interface ScheduleHeaderProps {
  titlebarPaddingClass: string;
  schedule: SchedulePageViewModel;
}

export function ScheduleHeader({
  titlebarPaddingClass,
  schedule,
}: ScheduleHeaderProps) {
  return (
    <header
      className={`shrink-0 flex items-center gap-3 py-2 pr-3 ${titlebarPaddingClass}`}
    >
      <div className="bg-primary rounded-md text-accent p-1.5">
        <Clock className="size-4" />
      </div>
      <div>
        <h1 className="text-base font-medium">Clockr</h1>
      </div>
      <Separator orientation="vertical" className="my-2" />
      <div className="grow" />
      <div className="app-no-drag flex flex-wrap items-center gap-2">
        {schedule.periods.length > 0 && (
          <Select
            value={String(schedule.activePeriod?.id ?? "")}
            onValueChange={(value) =>
              schedule.setSelectedPeriodId(Number(value))
            }
          >
            <SelectTrigger
              aria-label="Period"
              className="min-w-52 bg-white"
            >
              <SelectValue placeholder="Period" />
            </SelectTrigger>
            <SelectContent align="end">
              {schedule.periods.map((period) => (
                <SelectItem key={period.id} value={String(period.id)}>
                  {formatPeriodLabel(period)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}
        <ToggleGroup
          type="single"
          value={String(schedule.viewDayCount)}
          onValueChange={(value) => {
            if (value) {
              schedule.setViewDayCount(Number(value) as ScheduleViewDayCount);
            }
          }}
          variant="outline"
          spacing={0}
          aria-label="View length"
          className="bg-white"
        >
          {SCHEDULE_VIEW_DAY_OPTIONS.map((dayCount) => (
            <ToggleGroupItem
              key={dayCount}
              value={String(dayCount)}
              aria-label={`${dayCount} day view`}
            >
              {dayCount}d
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
        <Button
          type="button"
          disabled={
            !schedule.activePeriodId ||
            schedule.createPending ||
            schedule.days.length === 0
          }
          onClick={() =>
            schedule.handleCreate({
              day: schedule.days[0].date,
              startMinutes: 11 * 60,
              endMinutes: 12 * 60,
            })
          }
        >
          <Plus />
          {schedule.createPending ? "Saving" : "Add Block"}
        </Button>
      </div>
    </header>
  );
}
