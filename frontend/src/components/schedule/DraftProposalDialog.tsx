import { useEffect, useMemo, useState, type FormEvent } from "react";
import { CheckIcon, ScissorsIcon, XIcon } from "lucide-react";
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
import { Textarea } from "@/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { Category, Project, TimeEntry, TzSegment } from "@/lib/api";
import {
  OVERNIGHT_ATTRIBUTE_TO_START,
  OVERNIGHT_SPLIT_AT_MIDNIGHT,
  OVERLAP_ALLOW_PARALLEL,
  OVERLAP_KEEP_MINE,
  OVERLAP_KEEP_THEIRS,
  OVERLAP_SPLIT,
  activeTimeZoneForDay,
  entryCrossesLocalMidnight,
  findOverlappingConfirmed,
  isValidSplitCutPoints,
  toDate,
  zonedDateTimeParts,
  zonedDayMinutesToIso,
  type SchedulePlacement,
} from "@/lib/schedule";
import { TimeEntryAllocationFields } from "./TimeEntryAllocationFields";

const UNASSIGNED_CATEGORY_VALUE = "__unassigned__";

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

export interface DraftProposalEditValues {
  day: string;
  startMinutes: number;
  endMinutes: number;
  categoryId?: number;
  description: string;
  workType: string;
  projectId?: number;
  billableStatus: string;
}

interface DraftProposalDialogProps {
  open: boolean;
  draft: TimeEntry | null;
  placement?: SchedulePlacement | null;
  categories: Category[];
  projects: Project[];
  confirmedEntries: TimeEntry[];
  tzSegments: TzSegment[];
  isSaving: boolean;
  onOpenChange: (open: boolean) => void;
  onAdjust: (values: DraftProposalEditValues) => void;
  /** Persist form values before confirm when span is same-day editable. */
  onConfirm: (payload: {
    values?: DraftProposalEditValues;
    overnightPolicy?: string;
    overlapResolution?: string;
  }) => void;
  onReject: () => void;
  onSplit: (cutPoints: string[]) => void;
  actionError?: string | null;
}

export function DraftProposalDialog({
  open,
  draft,
  placement,
  categories,
  projects,
  confirmedEntries,
  tzSegments,
  isSaving,
  onOpenChange,
  onAdjust,
  onConfirm,
  onReject,
  onSplit,
  actionError,
}: DraftProposalDialogProps) {
  const [day, setDay] = useState("");
  const [startTime, setStartTime] = useState("09:00");
  const [endTime, setEndTime] = useState("10:00");
  const [categoryValue, setCategoryValue] = useState(UNASSIGNED_CATEGORY_VALUE);
  const [workType, setWorkType] = useState("worked");
  const [projectId, setProjectId] = useState<number | undefined>();
  const [billableStatus, setBillableStatus] = useState("unset");
  const [description, setDescription] = useState("");
  const [overnightPolicy, setOvernightPolicy] = useState(
    OVERNIGHT_ATTRIBUTE_TO_START,
  );
  const [overlapResolution, setOverlapResolution] = useState(OVERLAP_KEEP_MINE);
  const [cutTime, setCutTime] = useState("");
  const [cutPoints, setCutPoints] = useState<string[]>([]);
  const [formError, setFormError] = useState<string | null>(null);

  const timeZone = useMemo(() => {
    if (!draft) {
      return "UTC";
    }
    return activeTimeZoneForDay(
      day || draft.localWorkDate,
      tzSegments,
    );
  }, [day, draft, tzSegments]);

  const serverCrossesMidnight = useMemo(() => {
    if (!draft) {
      return false;
    }
    return entryCrossesLocalMidnight(draft.start, draft.end, timeZone);
  }, [draft, timeZone]);

  const formSpan = useMemo(() => {
    if (!draft) {
      return null;
    }
    // Same-day form cannot represent overnight spans — keep server interval.
    if (serverCrossesMidnight) {
      return { start: draft.start, end: draft.end };
    }
    const startMinutes = timeValueToMinutes(startTime);
    const endMinutes = timeValueToMinutes(endTime, true);
    if (startMinutes === null || endMinutes === null || !day) {
      return { start: draft.start, end: draft.end };
    }
    if (endMinutes <= startMinutes) {
      return { start: draft.start, end: draft.end };
    }
    const startIso = zonedDayMinutesToIso(day, startMinutes, timeZone);
    const endIso = zonedDayMinutesToIso(day, endMinutes, timeZone);
    if (!startIso || !endIso) {
      return { start: draft.start, end: draft.end };
    }
    return { start: startIso, end: endIso };
  }, [day, draft, endTime, serverCrossesMidnight, startTime, timeZone]);

  const crossesMidnight = serverCrossesMidnight;

  const overlaps = useMemo(() => {
    if (!draft || !formSpan) {
      return [];
    }
    return findOverlappingConfirmed(
      { id: draft.id, start: formSpan.start, end: formSpan.end },
      confirmedEntries,
    );
  }, [confirmedEntries, draft, formSpan]);

  useEffect(() => {
    if (!draft) {
      return;
    }

    const startMinutes = placement?.startMinutes;
    const endMinutes = placement?.endMinutes;
    setDay(placement?.day ?? draft.localWorkDate);
    if (typeof startMinutes === "number" && typeof endMinutes === "number") {
      setStartTime(minutesToTimeValue(startMinutes));
      setEndTime(minutesToTimeValue(endMinutes));
    } else {
      const startDate = toDate(draft.start);
      const endDate = toDate(draft.end);
      if (startDate && endDate) {
        setStartTime(
          minutesToTimeValue(zonedDateTimeParts(startDate, timeZone).minutes),
        );
        setEndTime(
          minutesToTimeValue(zonedDateTimeParts(endDate, timeZone).minutes),
        );
      } else {
        setStartTime("09:00");
        setEndTime("10:00");
      }
    }
    setCategoryValue(
      typeof draft.categoryId === "number"
        ? draft.categoryId.toString()
        : UNASSIGNED_CATEGORY_VALUE,
    );
    setWorkType(draft.workType || "worked");
    setProjectId(draft.projectId);
    setBillableStatus(draft.billableStatus || "unset");
    setDescription(draft.description ?? "");
    setOvernightPolicy(OVERNIGHT_ATTRIBUTE_TO_START);
    setOverlapResolution(OVERLAP_KEEP_MINE);
    setCutTime("");
    setCutPoints([]);
    setFormError(null);
  }, [draft, placement, timeZone]);

  const parseFormValues = (): DraftProposalEditValues | null => {
    const startMinutes = timeValueToMinutes(startTime);
    const endMinutes = timeValueToMinutes(endTime, true);
    if (startMinutes === null || endMinutes === null) {
      setFormError("Use a valid start and end time.");
      return null;
    }
    if (endMinutes <= startMinutes) {
      setFormError("End time must be after start time.");
      return null;
    }
    if (!day) {
      setFormError("Choose a date.");
      return null;
    }

    return {
      day,
      startMinutes,
      endMinutes,
      categoryId:
        categoryValue === UNASSIGNED_CATEGORY_VALUE
          ? undefined
          : Number(categoryValue),
      description: description.trim(),
      workType,
      projectId,
      billableStatus,
    };
  };

  const handleAdjust = (submitEvent: FormEvent<HTMLFormElement>) => {
    submitEvent.preventDefault();
    const values = parseFormValues();
    if (!values) {
      return;
    }
    setFormError(null);
    onAdjust(values);
  };

  const handleConfirm = () => {
    if (!draft || !formSpan) {
      return;
    }
    setFormError(null);

    const policies = {
      ...(crossesMidnight ? { overnightPolicy } : {}),
      ...(overlaps.length > 0 ? { overlapResolution } : {}),
    };

    // Overnight drafts keep server span; same-day form adjusts first.
    if (serverCrossesMidnight) {
      onConfirm(policies);
      return;
    }

    const values = parseFormValues();
    if (!values) {
      return;
    }
    onConfirm({ values, ...policies });
  };

  const handleAddCut = () => {
    if (!draft || !formSpan) {
      return;
    }
    const minutes = timeValueToMinutes(cutTime);
    if (minutes === null) {
      setFormError("Use a valid cut time.");
      return;
    }
    const cutDay = day || draft.localWorkDate;
    const iso = zonedDayMinutesToIso(cutDay, minutes, timeZone);
    if (!iso) {
      setFormError("Could not resolve cut time in the active timezone.");
      return;
    }
    const next = [...cutPoints, iso].sort();
    if (!isValidSplitCutPoints(formSpan.start, formSpan.end, next)) {
      setFormError("Cut points must sit strictly inside the draft span.");
      return;
    }
    setCutPoints(next);
    setCutTime("");
    setFormError(null);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Draft proposal</DialogTitle>
          <DialogDescription>
            Adjust, confirm, split, or reject this draft time entry before it
            becomes payable.
          </DialogDescription>
        </DialogHeader>

        <form className="space-y-4" onSubmit={handleAdjust}>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="draft-day">Day</FieldLabel>
              <Input
                id="draft-day"
                type="date"
                value={day}
                onChange={(event) => setDay(event.target.value)}
              />
            </Field>
            <div className="grid grid-cols-2 gap-3">
              <Field>
                <FieldLabel htmlFor="draft-start">Start</FieldLabel>
                <Input
                  id="draft-start"
                  type="time"
                  value={startTime}
                  onChange={(event) => setStartTime(event.target.value)}
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="draft-end">End</FieldLabel>
                <Input
                  id="draft-end"
                  type="time"
                  value={endTime}
                  onChange={(event) => setEndTime(event.target.value)}
                />
              </Field>
            </div>
            <Field>
              <FieldLabel htmlFor="draft-category">Category</FieldLabel>
              <Select value={categoryValue} onValueChange={setCategoryValue}>
                <SelectTrigger id="draft-category" className="w-full">
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
            <Field>
              <FieldLabel htmlFor="draft-description">Description</FieldLabel>
              <Textarea
                id="draft-description"
                value={description}
                onChange={(event) => setDescription(event.target.value)}
                rows={2}
              />
            </Field>
            <TimeEntryAllocationFields
              idPrefix="draft"
              projects={projects}
              values={{ workType, projectId, billableStatus }}
              onChange={(values) => {
                setWorkType(values.workType);
                setProjectId(values.projectId);
                setBillableStatus(values.billableStatus);
              }}
            />
          </FieldGroup>

          {crossesMidnight ? (
            <Field>
              <FieldLabel>Overnight policy</FieldLabel>
              <div className="space-y-2 text-sm">
                <label className="flex items-center gap-2">
                  <input
                    type="radio"
                    name="overnight-policy"
                    checked={overnightPolicy === OVERNIGHT_ATTRIBUTE_TO_START}
                    onChange={() =>
                      setOvernightPolicy(OVERNIGHT_ATTRIBUTE_TO_START)
                    }
                  />
                  Attribute to start day
                </label>
                <label className="flex items-center gap-2">
                  <input
                    type="radio"
                    name="overnight-policy"
                    checked={overnightPolicy === OVERNIGHT_SPLIT_AT_MIDNIGHT}
                    onChange={() =>
                      setOvernightPolicy(OVERNIGHT_SPLIT_AT_MIDNIGHT)
                    }
                  />
                  Split at midnight
                </label>
              </div>
            </Field>
          ) : null}

          {overlaps.length > 0 ? (
            <Field>
              <FieldLabel>
                Overlap resolution ({overlaps.length} confirmed)
              </FieldLabel>
              <div className="space-y-2 text-sm">
                {(
                  [
                    [OVERLAP_KEEP_MINE, "Keep mine (delete theirs)"],
                    [OVERLAP_KEEP_THEIRS, "Keep theirs (dismiss draft)"],
                    [OVERLAP_SPLIT, "Split around overlaps"],
                    [OVERLAP_ALLOW_PARALLEL, "Allow parallel"],
                  ] as const
                ).map(([value, label]) => (
                  <label key={value} className="flex items-center gap-2">
                    <input
                      type="radio"
                      name="overlap-resolution"
                      checked={overlapResolution === value}
                      onChange={() => setOverlapResolution(value)}
                    />
                    {label}
                  </label>
                ))}
              </div>
            </Field>
          ) : null}

          <Field>
            <FieldLabel htmlFor="draft-cut">Split cut points</FieldLabel>
            <div className="flex gap-2">
              <Input
                id="draft-cut"
                type="time"
                value={cutTime}
                onChange={(event) => setCutTime(event.target.value)}
              />
              <Button type="button" variant="outline" onClick={handleAddCut}>
                Add cut
              </Button>
            </div>
            {cutPoints.length > 0 ? (
              <p className="mt-1 text-xs text-muted-foreground">
                {cutPoints.length} cut point{cutPoints.length === 1 ? "" : "s"}{" "}
                ready
              </p>
            ) : null}
          </Field>

          {formError ? <FieldError>{formError}</FieldError> : null}
          {actionError ? <FieldError>{actionError}</FieldError> : null}

          <DialogFooter className="flex-col gap-2 sm:flex-row sm:justify-between">
            <div className="flex flex-wrap gap-2">
              <Button
                type="button"
                variant="destructive"
                disabled={isSaving}
                onClick={onReject}
              >
                <XIcon />
                Reject
              </Button>
              <Button
                type="button"
                variant="outline"
                disabled={isSaving || cutPoints.length === 0}
                onClick={() => onSplit(cutPoints)}
              >
                <ScissorsIcon />
                Split
              </Button>
            </div>
            <div className="flex flex-wrap gap-2">
              <Button type="submit" variant="secondary" disabled={isSaving}>
                Save adjust
              </Button>
              <Button type="button" disabled={isSaving} onClick={handleConfirm}>
                <CheckIcon />
                Confirm
              </Button>
            </div>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
