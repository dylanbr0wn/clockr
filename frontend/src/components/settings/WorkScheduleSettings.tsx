import { LoaderCircle, Pencil, Plus, Trash2 } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
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
  FieldLabel,
} from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import {
  Item,
  ItemActions,
  ItemContent,
  ItemDescription,
  ItemGroup,
  ItemTitle,
} from "@/components/ui/item";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  useActiveWorkSchedule,
  useDeleteScheduleException,
  useReplaceActiveWorkSchedule,
  useScheduleExceptions,
  useUpsertScheduleException,
} from "@/lib/api";
import type {
  ScheduleException,
  WorkSchedule,
  WorkScheduleDay,
  WorkingWindow,
} from "@/lib/api/types";
import { formatMinutes } from "@/lib/scheduler";
import { SettingBlock } from "./SettingBlock";

const WEEKDAYS = [
  "monday",
  "tuesday",
  "wednesday",
  "thursday",
  "friday",
  "saturday",
  "sunday",
] as const;

type Weekday = (typeof WEEKDAYS)[number];

const WEEKDAY_LABELS: Record<Weekday, string> = {
  monday: "Monday",
  tuesday: "Tuesday",
  wednesday: "Wednesday",
  thursday: "Thursday",
  friday: "Friday",
  saturday: "Saturday",
  sunday: "Sunday",
};

const EXCEPTION_KINDS = ["holiday", "leave", "changed_hours"] as const;
type ExceptionKind = (typeof EXCEPTION_KINDS)[number];

interface ScheduleDraft {
  timezone: string;
  workweekStart: Weekday;
  days: WorkScheduleDay[];
}

interface ExceptionDraft {
  date: string;
  kind: ExceptionKind;
  expectedHours: string;
  windowStart: string;
  windowEnd: string;
}

function emptyDays(): WorkScheduleDay[] {
  return WEEKDAYS.map((weekday) => ({
    weekday,
    expectedMinutes: 0,
    windows: [],
  }));
}

function draftFromSchedule(schedule: WorkSchedule | null | undefined): ScheduleDraft {
  const byWeekday = new Map(
    (schedule?.days ?? []).map((day) => [day.weekday, day]),
  );
  return {
    timezone: schedule?.timezone ?? "UTC",
    workweekStart: (schedule?.workweekStart as Weekday) || "monday",
    days: WEEKDAYS.map((weekday) => {
      const existing = byWeekday.get(weekday);
      return {
        weekday,
        expectedMinutes: existing?.expectedMinutes ?? 0,
        windows: existing?.windows ?? [],
      };
    }),
  };
}

function emptyExceptionDraft(): ExceptionDraft {
  return {
    date: "",
    kind: "holiday",
    expectedHours: "0",
    windowStart: "09:00",
    windowEnd: "17:00",
  };
}

function draftFromException(exception: ScheduleException): ExceptionDraft {
  const window = exception.windows[0];
  return {
    date: exception.date,
    kind: (EXCEPTION_KINDS.includes(exception.kind as ExceptionKind)
      ? exception.kind
      : "holiday") as ExceptionKind,
    expectedHours: String(exception.expectedMinutes / 60),
    windowStart: window ? formatMinutes(window.startMinutes) : "09:00",
    windowEnd: window ? formatMinutes(window.endMinutes) : "17:00",
  };
}

function todayInTimeZone(timeZone: string): string {
  try {
    return new Intl.DateTimeFormat("en-CA", {
      timeZone,
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
    }).format(new Date());
  } catch {
    return new Intl.DateTimeFormat("en-CA", {
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
    }).format(new Date());
  }
}

function parseHoursToMinutes(raw: string): number | null {
  if (raw.trim() === "") {
    return null;
  }
  const parsed = Number(raw);
  if (!Number.isFinite(parsed) || parsed < 0 || parsed > 24) {
    return null;
  }
  return Math.round(parsed * 60);
}

function parseTimeToMinutes(raw: string): number | null {
  const match = /^(\d{2}):(\d{2})$/.exec(raw);
  if (!match) {
    return null;
  }
  const hours = Number(match[1]);
  const minutes = Number(match[2]);
  if (hours < 0 || hours > 23 || minutes < 0 || minutes > 59) {
    return null;
  }
  return hours * 60 + minutes;
}

function hoursDisplay(minutes: number): string {
  const hours = minutes / 60;
  return Number.isInteger(hours) ? String(hours) : String(hours);
}

function DayHoursInput({
  weekday,
  label,
  minutes,
  onCommit,
}: {
  weekday: Weekday;
  label: string;
  minutes: number;
  onCommit: (rawHours: string) => void;
}) {
  const [draft, setDraft] = useState(hoursDisplay(minutes));

  useEffect(() => {
    setDraft(hoursDisplay(minutes));
  }, [minutes]);

  return (
    <Field>
      <FieldLabel htmlFor={`setting-${weekday}-hours`}>{label}</FieldLabel>
      <Input
        id={`setting-${weekday}-hours`}
        type="number"
        min={0}
        max={24}
        step={0.25}
        value={draft}
        onChange={(event) => setDraft(event.target.value)}
        onBlur={() => onCommit(draft)}
        className="bg-background"
      />
    </Field>
  );
}

export function WorkScheduleSettings() {
  const scheduleQuery = useActiveWorkSchedule();
  const exceptionsQuery = useScheduleExceptions();
  const replaceSchedule = useReplaceActiveWorkSchedule();
  const upsertException = useUpsertScheduleException();
  const deleteException = useDeleteScheduleException();

  const [draft, setDraft] = useState<ScheduleDraft>(() =>
    draftFromSchedule(scheduleQuery.data),
  );
  const [scheduleError, setScheduleError] = useState<string | null>(null);
  const [exceptionError, setExceptionError] = useState<string | null>(null);
  const [editorOpen, setEditorOpen] = useState(false);
  const [editingException, setEditingException] =
    useState<ScheduleException | null>(null);
  const [exceptionDraft, setExceptionDraft] = useState<ExceptionDraft>(
    emptyExceptionDraft,
  );

  useEffect(() => {
    if (scheduleQuery.data) {
      setDraft(draftFromSchedule(scheduleQuery.data));
    }
  }, [scheduleQuery.data]);

  const exceptions = useMemo(
    () =>
      [...(exceptionsQuery.data ?? [])].sort((a, b) =>
        a.date.localeCompare(b.date),
      ),
    [exceptionsQuery.data],
  );

  const isLoading = scheduleQuery.isLoading || exceptionsQuery.isLoading;
  const isSaving =
    replaceSchedule.isPending ||
    upsertException.isPending ||
    deleteException.isPending;

  const updateDayHours = (weekday: Weekday, rawHours: string) => {
    setDraft((current) => ({
      ...current,
      days: current.days.map((day) => {
        if (day.weekday !== weekday) {
          return day;
        }
        const minutes = parseHoursToMinutes(rawHours);
        if (minutes === null) {
          return day;
        }
        const windows =
          minutes === 0
            ? []
            : day.windows.length > 0
              ? day.windows
              : [{ startMinutes: 9 * 60, endMinutes: 17 * 60 }];
        return { ...day, expectedMinutes: minutes, windows };
      }),
    }));
  };

  const updateDayWindow = (
    weekday: Weekday,
    field: "startMinutes" | "endMinutes",
    rawTime: string,
  ) => {
    const minutes = parseTimeToMinutes(rawTime);
    if (minutes === null) {
      return;
    }
    setDraft((current) => ({
      ...current,
      days: current.days.map((day) => {
        if (day.weekday !== weekday) {
          return day;
        }
        const existing = day.windows[0] ?? {
          startMinutes: 9 * 60,
          endMinutes: 17 * 60,
        };
        const next: WorkingWindow = { ...existing, [field]: minutes };
        return { ...day, windows: day.expectedMinutes > 0 ? [next] : [] };
      }),
    }));
  };

  const handleSaveSchedule = async () => {
    if (!draft.timezone.trim()) {
      setScheduleError("Timezone is required.");
      return;
    }
    setScheduleError(null);
    try {
      await replaceSchedule.mutateAsync({
        timezone: draft.timezone.trim(),
        workweekStart: draft.workweekStart,
        effectiveFrom: todayInTimeZone(draft.timezone.trim()),
        days: draft.days,
      });
    } catch (error) {
      setScheduleError(
        error instanceof Error ? error.message : "Could not save schedule",
      );
    }
  };

  const openCreateException = () => {
    setEditingException(null);
    setExceptionDraft(emptyExceptionDraft());
    setExceptionError(null);
    setEditorOpen(true);
  };

  const openEditException = (exception: ScheduleException) => {
    setEditingException(exception);
    setExceptionDraft(draftFromException(exception));
    setExceptionError(null);
    setEditorOpen(true);
  };

  const handleSaveException = async () => {
    if (!/^\d{4}-\d{2}-\d{2}$/.test(exceptionDraft.date)) {
      setExceptionError("Date is required (YYYY-MM-DD).");
      return;
    }

    let expectedMinutes = 0;
    let windows: WorkingWindow[] = [];

    if (exceptionDraft.kind === "changed_hours") {
      const minutes = parseHoursToMinutes(exceptionDraft.expectedHours);
      if (minutes === null) {
        setExceptionError("Expected hours must be between 0 and 24.");
        return;
      }
      expectedMinutes = minutes;
      if (minutes > 0) {
        const start = parseTimeToMinutes(exceptionDraft.windowStart);
        const end = parseTimeToMinutes(exceptionDraft.windowEnd);
        if (start === null || end === null || end <= start) {
          setExceptionError("Window end must be after start.");
          return;
        }
        windows = [{ startMinutes: start, endMinutes: end }];
      }
    }

    setExceptionError(null);
    try {
      await upsertException.mutateAsync({
        date: exceptionDraft.date,
        kind: exceptionDraft.kind,
        expectedMinutes,
        windows,
      });
      setEditorOpen(false);
    } catch (error) {
      setExceptionError(
        error instanceof Error ? error.message : "Could not save exception",
      );
    }
  };

  const handleDeleteException = async (date: string) => {
    setExceptionError(null);
    try {
      await deleteException.mutateAsync(date);
    } catch (error) {
      setExceptionError(
        error instanceof Error ? error.message : "Could not delete exception",
      );
    }
  };

  if (isLoading && !scheduleQuery.data) {
    return (
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <LoaderCircle className="size-4 animate-spin" />
        Loading work schedule…
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <SettingBlock
        title="Weekday template"
        description="Expected minutes and optional working windows for each weekday. Saving starts a new effective range from today."
      >
        <div className="grid gap-3 sm:grid-cols-2">
          <Field>
            <FieldLabel htmlFor="setting-schedule-timezone">Timezone</FieldLabel>
            <Input
              id="setting-schedule-timezone"
              value={draft.timezone}
              onChange={(event) =>
                setDraft((current) => ({
                  ...current,
                  timezone: event.target.value,
                }))
              }
              className="bg-background font-mono text-sm"
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="setting-workweek-start">
              Workweek starts
            </FieldLabel>
            <Select
              value={draft.workweekStart}
              onValueChange={(value) =>
                setDraft((current) => ({
                  ...current,
                  workweekStart: value as Weekday,
                }))
              }
            >
              <SelectTrigger
                id="setting-workweek-start"
                className="w-full bg-background"
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {WEEKDAYS.map((weekday) => (
                  <SelectItem key={weekday} value={weekday}>
                    {WEEKDAY_LABELS[weekday]}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>
        </div>

        <div className="space-y-3">
          {draft.days.map((day) => {
            const weekday = day.weekday as Weekday;
            const window = day.windows[0];
            return (
              <div
                key={weekday}
                className="grid gap-2 sm:grid-cols-[7rem_minmax(0,1fr)_minmax(0,1fr)_minmax(0,1fr)] sm:items-end"
              >
                <DayHoursInput
                  weekday={weekday}
                  label={`${WEEKDAY_LABELS[weekday]} hours`}
                  minutes={day.expectedMinutes}
                  onCommit={(raw) => updateDayHours(weekday, raw)}
                />
                <Field>
                  <FieldLabel htmlFor={`setting-${weekday}-window-start`}>
                    Window start
                  </FieldLabel>
                  <Input
                    id={`setting-${weekday}-window-start`}
                    type="time"
                    disabled={day.expectedMinutes === 0}
                    value={
                      window ? formatMinutes(window.startMinutes) : "09:00"
                    }
                    onChange={(event) =>
                      updateDayWindow(
                        weekday,
                        "startMinutes",
                        event.target.value,
                      )
                    }
                    className="bg-background"
                  />
                </Field>
                <Field>
                  <FieldLabel htmlFor={`setting-${weekday}-window-end`}>
                    Window end
                  </FieldLabel>
                  <Input
                    id={`setting-${weekday}-window-end`}
                    type="time"
                    disabled={day.expectedMinutes === 0}
                    value={
                      window ? formatMinutes(window.endMinutes) : "17:00"
                    }
                    onChange={(event) =>
                      updateDayWindow(
                        weekday,
                        "endMinutes",
                        event.target.value,
                      )
                    }
                    className="bg-background"
                  />
                </Field>
              </div>
            );
          })}
        </div>

        <div className="flex items-center gap-3">
          <Button
            type="button"
            onClick={() => {
              void handleSaveSchedule();
            }}
            disabled={isSaving}
          >
            {replaceSchedule.isPending ? (
              <LoaderCircle className="size-4 animate-spin" />
            ) : (
              "Save schedule"
            )}
          </Button>
          {scheduleError ? <FieldError>{scheduleError}</FieldError> : null}
        </div>
      </SettingBlock>

      <SettingBlock
        title="Exceptions"
        description="Date-keyed overrides: holidays, leave, or changed hours. Windows apply only to changed hours."
      >
        <div className="flex justify-end">
          <Button type="button" size="sm" onClick={openCreateException}>
            <Plus className="size-4" />
            Add exception
          </Button>
        </div>

        {exceptions.length === 0 ? (
          <p className="text-sm text-muted-foreground">No exceptions yet.</p>
        ) : (
          <ItemGroup>
            {exceptions.map((exception) => (
              <Item key={exception.id} variant="outline" size="sm">
                <ItemContent>
                  <ItemTitle>{exception.date}</ItemTitle>
                  <ItemDescription>
                    {exception.kind.replace("_", " ")}
                    {exception.kind === "changed_hours"
                      ? ` · ${hoursDisplay(exception.expectedMinutes)}h`
                      : null}
                  </ItemDescription>
                </ItemContent>
                <ItemActions>
                  <Button
                    type="button"
                    size="icon-sm"
                    variant="ghost"
                    aria-label={`Edit exception ${exception.date}`}
                    onClick={() => openEditException(exception)}
                  >
                    <Pencil className="size-4" />
                  </Button>
                  <Button
                    type="button"
                    size="icon-sm"
                    variant="ghost"
                    aria-label={`Delete exception ${exception.date}`}
                    onClick={() => {
                      void handleDeleteException(exception.date);
                    }}
                    disabled={deleteException.isPending}
                  >
                    <Trash2 className="size-4" />
                  </Button>
                </ItemActions>
              </Item>
            ))}
          </ItemGroup>
        )}
        {exceptionError && !editorOpen ? (
          <FieldError>{exceptionError}</FieldError>
        ) : null}
      </SettingBlock>

      <p className="text-xs text-muted-foreground">
        {isSaving ? "Saving" : isLoading ? "Loading" : "Ready"}
      </p>

      <Dialog open={editorOpen} onOpenChange={setEditorOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {editingException ? "Edit exception" : "Add exception"}
            </DialogTitle>
            <DialogDescription>
              Overrides expected time for a single local date.
            </DialogDescription>
          </DialogHeader>

          <div className="grid gap-3">
            <Field>
              <FieldLabel htmlFor="exception-date">Date</FieldLabel>
              <Input
                id="exception-date"
                type="date"
                value={exceptionDraft.date}
                onChange={(event) =>
                  setExceptionDraft((current) => ({
                    ...current,
                    date: event.target.value,
                  }))
                }
                disabled={Boolean(editingException)}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="exception-kind">Kind</FieldLabel>
              <Select
                value={exceptionDraft.kind}
                onValueChange={(value) =>
                  setExceptionDraft((current) => ({
                    ...current,
                    kind: value as ExceptionKind,
                    expectedHours:
                      value === "changed_hours" ? current.expectedHours : "0",
                  }))
                }
              >
                <SelectTrigger id="exception-kind" className="w-full bg-background">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="holiday">Holiday</SelectItem>
                  <SelectItem value="leave">Leave</SelectItem>
                  <SelectItem value="changed_hours">Changed hours</SelectItem>
                </SelectContent>
              </Select>
            </Field>
            {exceptionDraft.kind === "changed_hours" ? (
              <>
                <Field>
                  <FieldLabel htmlFor="exception-hours">Expected hours</FieldLabel>
                  <Input
                    id="exception-hours"
                    type="number"
                    min={0}
                    max={24}
                    step={0.25}
                    value={exceptionDraft.expectedHours}
                    onChange={(event) =>
                      setExceptionDraft((current) => ({
                        ...current,
                        expectedHours: event.target.value,
                      }))
                    }
                  />
                </Field>
                <div className="grid gap-3 sm:grid-cols-2">
                  <Field>
                    <FieldLabel htmlFor="exception-window-start">
                      Window start
                    </FieldLabel>
                    <Input
                      id="exception-window-start"
                      type="time"
                      value={exceptionDraft.windowStart}
                      onChange={(event) =>
                        setExceptionDraft((current) => ({
                          ...current,
                          windowStart: event.target.value,
                        }))
                      }
                    />
                  </Field>
                  <Field>
                    <FieldLabel htmlFor="exception-window-end">
                      Window end
                    </FieldLabel>
                    <Input
                      id="exception-window-end"
                      type="time"
                      value={exceptionDraft.windowEnd}
                      onChange={(event) =>
                        setExceptionDraft((current) => ({
                          ...current,
                          windowEnd: event.target.value,
                        }))
                      }
                    />
                  </Field>
                </div>
              </>
            ) : null}
            {exceptionError ? <FieldError>{exceptionError}</FieldError> : null}
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => setEditorOpen(false)}
            >
              Cancel
            </Button>
            <Button
              type="button"
              onClick={() => {
                void handleSaveException();
              }}
              disabled={upsertException.isPending}
            >
              {upsertException.isPending ? (
                <LoaderCircle className="size-4 animate-spin" />
              ) : (
                "Save exception"
              )}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
