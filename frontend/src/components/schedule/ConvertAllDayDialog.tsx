import { useEffect, useState, type FormEvent } from "react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Field,
  FieldError,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import type { AllDayChip } from "@/lib/schedule";

function minutesToTimeValue(minutes: number) {
  const hours = Math.floor(minutes / 60)
    .toString()
    .padStart(2, "0");
  const mins = (minutes % 60).toString().padStart(2, "0");
  return `${hours}:${mins}`;
}

function timeValueToMinutes(value: string) {
  const [hoursValue, minutesValue] = value.split(":");
  const hours = Number(hoursValue);
  const minutes = Number(minutesValue);
  if (
    !Number.isInteger(hours) ||
    !Number.isInteger(minutes) ||
    hours < 0 ||
    hours > 23 ||
    minutes < 0 ||
    minutes > 59
  ) {
    return null;
  }
  return hours * 60 + minutes;
}

export interface ConvertAllDayValues {
  day: string;
  startMinutes: number;
  endMinutes: number;
  categoryId?: number;
  description: string;
}

interface ConvertAllDayDialogProps {
  open: boolean;
  chip: AllDayChip | null;
  isSaving: boolean;
  onOpenChange: (open: boolean) => void;
  onConvert: (values: ConvertAllDayValues) => void;
}

export function ConvertAllDayDialog({
  open,
  chip,
  isSaving,
  onOpenChange,
  onConvert,
}: ConvertAllDayDialogProps) {
  const [startTime, setStartTime] = useState("09:00");
  const [endTime, setEndTime] = useState("10:00");
  const [formError, setFormError] = useState<string | null>(null);

  useEffect(() => {
    if (!chip) {
      return;
    }
    setStartTime(minutesToTimeValue(9 * 60));
    setEndTime(minutesToTimeValue(10 * 60));
    setFormError(null);
  }, [chip]);

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!chip) {
      return;
    }
    const startMinutes = timeValueToMinutes(startTime);
    const endMinutes = timeValueToMinutes(endTime);
    if (startMinutes === null || endMinutes === null) {
      setFormError("Use a valid start and end time.");
      return;
    }
    if (endMinutes <= startMinutes) {
      setFormError("End time must be after start time.");
      return;
    }

    onConvert({
      day: chip.day,
      startMinutes,
      endMinutes,
      categoryId: chip.categoryId,
      description: chip.title,
    });
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Convert to time entry</DialogTitle>
          <DialogDescription>
            Pick a timed interval on {chip?.day ?? "this day"} for “
            {chip?.title ?? "event"}”. The result opens as a draft proposal.
          </DialogDescription>
        </DialogHeader>
        <form className="space-y-4" onSubmit={handleSubmit}>
          <FieldGroup>
            <div className="grid grid-cols-2 gap-3">
              <Field>
                <FieldLabel htmlFor="convert-start">Start</FieldLabel>
                <Input
                  id="convert-start"
                  type="time"
                  value={startTime}
                  onChange={(event) => setStartTime(event.target.value)}
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="convert-end">End</FieldLabel>
                <Input
                  id="convert-end"
                  type="time"
                  value={endTime}
                  onChange={(event) => setEndTime(event.target.value)}
                />
              </Field>
            </div>
          </FieldGroup>
          {formError ? <FieldError>{formError}</FieldError> : null}
          <DialogFooter>
            <Button
              type="button"
              variant="ghost"
              onClick={() => onOpenChange(false)}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={isSaving || !chip}>
              Convert
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
