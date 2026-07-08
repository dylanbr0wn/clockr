import { useEffect, useState, type FormEvent } from "react";
import { format } from "date-fns";
import { CalendarIcon, SaveIcon } from "lucide-react";
import { Button, buttonVariants } from "@/components/ui/button";
import { Calendar } from "@/components/ui/calendar";
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
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { Category } from "@/lib/api";
import { formatMinutes } from "@/lib/scheduler";
import { cn } from "@/lib/utils";
import type {
  EditableScheduleEvent,
  ScheduleEventEditValues,
} from "./useSchedulePage";

const UNASSIGNED_CATEGORY_VALUE = "__unassigned__";

function dateStringToDate(value: string) {
  const [yearValue, monthValue, dayValue] = value.split("-");
  const year = Number(yearValue);
  const month = Number(monthValue);
  const date = Number(dayValue);

  if (
    !Number.isInteger(year) ||
    !Number.isInteger(month) ||
    !Number.isInteger(date)
  ) {
    return undefined;
  }

  return new Date(year, month - 1, date);
}

function dateToDateString(date: Date) {
  const year = date.getFullYear();
  const month = (date.getMonth() + 1).toString().padStart(2, "0");
  const day = date.getDate().toString().padStart(2, "0");

  return `${year}-${month}-${day}`;
}

function minutesToTimeValue(minutes: number) {
  const hours = Math.floor(minutes / 60)
    .toString()
    .padStart(2, "0");
  const mins = (minutes % 60).toString().padStart(2, "0");

  return `${hours}:${mins}`;
}

function timeValueToMinutes(value: string, allowEndOfDay = false) {
  const [hoursValue, minutesValue] = value.split(":");
  const hours = Number(hoursValue);
  const minutes = Number(minutesValue);

  if (allowEndOfDay && hours === 24 && minutes === 0) {
    return 24 * 60;
  }

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

interface EventEditDialogProps {
  categories: Category[];
  event: EditableScheduleEvent | null;
  isSaving: boolean;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSave: (values: ScheduleEventEditValues) => void;
}

export function EventEditDialog({
  categories,
  event,
  isSaving,
  open,
  onOpenChange,
  onSave,
}: EventEditDialogProps) {
  const [day, setDay] = useState("");
  const [startTime, setStartTime] = useState("09:00");
  const [endTime, setEndTime] = useState("10:00");
  const [categoryValue, setCategoryValue] = useState(
    UNASSIGNED_CATEGORY_VALUE,
  );
  const [title, setTitle] = useState("");
  const [formError, setFormError] = useState<string | null>(null);
  const [datePickerOpen, setDatePickerOpen] = useState(false);
  const selectedDate = dateStringToDate(day);

  useEffect(() => {
    if (!event) {
      return;
    }

    setDay(event.day);
    setStartTime(minutesToTimeValue(event.startMinutes));
    setEndTime(minutesToTimeValue(event.endMinutes));
    setCategoryValue(
      typeof event.categoryId === "number"
        ? event.categoryId.toString()
        : UNASSIGNED_CATEGORY_VALUE,
    );
    setTitle(event.note);
    setFormError(null);
    setDatePickerOpen(false);
  }, [event]);

  const handleSubmit = (submitEvent: FormEvent<HTMLFormElement>) => {
    submitEvent.preventDefault();

    const startMinutes = timeValueToMinutes(startTime);
    const endMinutes = timeValueToMinutes(endTime, true);

    if (startMinutes === null || endMinutes === null) {
      setFormError("Use a valid start and end time.");
      return;
    }

    if (endMinutes <= startMinutes) {
      setFormError("End time must be after start time.");
      return;
    }

    if (!day) {
      setFormError("Choose a date.");
      return;
    }

    onSave({
      day,
      startMinutes,
      endMinutes,
      categoryId:
        categoryValue === UNASSIGNED_CATEGORY_VALUE
          ? undefined
          : Number(categoryValue),
      note: title.trim(),
    });
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{event?.isNew ? "New event" : "Edit event"}</DialogTitle>
          <DialogDescription>
            {event
              ? event.isNew
                ? `${formatMinutes(event.startMinutes)}-${formatMinutes(
                    event.endMinutes,
                  )}`
                : `${event.category} · ${formatMinutes(
                    event.startMinutes,
                  )}-${formatMinutes(event.endMinutes)}`
              : "Update the selected event."}
          </DialogDescription>
        </DialogHeader>

        <form className="grid gap-4" onSubmit={handleSubmit}>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="event-title">Title</FieldLabel>
              <Input
                id="event-title"
                value={title}
                onChange={(changeEvent) => setTitle(changeEvent.target.value)}
                placeholder={event?.category ?? "Unassigned"}
              />
            </Field>

            <div className="grid gap-3 sm:grid-cols-3">
              <Field>
                <FieldLabel htmlFor="event-date">Date</FieldLabel>
                <Popover
                  modal
                  open={datePickerOpen}
                  onOpenChange={setDatePickerOpen}
                >
                  <PopoverTrigger
                    id="event-date"
                    type="button"
                    data-empty={!selectedDate}
                    className={cn(
                      buttonVariants({ variant: "outline" }),
                      "w-full justify-start text-left font-normal data-[empty=true]:text-muted-foreground",
                    )}
                  >
                    <CalendarIcon data-icon="inline-start" />
                    {selectedDate ? format(selectedDate, "PPP") : "Pick a date"}
                  </PopoverTrigger>
                  <PopoverContent className="w-auto p-0" align="start">
                    <Calendar
                      mode="single"
                      selected={selectedDate}
                      defaultMonth={selectedDate}
                      onSelect={(nextDate) => {
                        if (nextDate) {
                          setDay(dateToDateString(nextDate));
                          setDatePickerOpen(false);
                        }
                      }}
                    />
                  </PopoverContent>
                </Popover>
              </Field>
              <Field>
                <FieldLabel htmlFor="event-start">Start</FieldLabel>
                <Input
                  id="event-start"
                  type="text"
                  inputMode="numeric"
                  placeholder="09:00"
                  value={startTime}
                  onChange={(changeEvent) =>
                    setStartTime(changeEvent.target.value)
                  }
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="event-end">End</FieldLabel>
                <Input
                  id="event-end"
                  type="text"
                  inputMode="numeric"
                  placeholder="17:00"
                  value={endTime}
                  onChange={(changeEvent) =>
                    setEndTime(changeEvent.target.value)
                  }
                />
              </Field>
            </div>

            <Field>
              <FieldLabel htmlFor="event-category">Category</FieldLabel>
              <Select value={categoryValue} onValueChange={setCategoryValue}>
                <SelectTrigger id="event-category" className="w-full">
                  <SelectValue placeholder="Unassigned" />
                </SelectTrigger>
                <SelectContent position="popper" align="start">
                  <SelectItem value={UNASSIGNED_CATEGORY_VALUE}>
                    Unassigned
                  </SelectItem>
                  {categories.map((category) => (
                    <SelectItem
                      key={category.id}
                      value={category.id.toString()}
                    >
                      {category.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </Field>

            {formError ? <FieldError>{formError}</FieldError> : null}
          </FieldGroup>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={isSaving}>
              <SaveIcon data-icon="inline-start" />
              {isSaving
                ? event?.isNew
                  ? "Creating"
                  : "Saving"
                : event?.isNew
                  ? "Create"
                  : "Save"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
