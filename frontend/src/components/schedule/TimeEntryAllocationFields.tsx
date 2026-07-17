import {
  Field,
  FieldLabel,
} from "@/components/ui/field";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { Project } from "@/lib/api";
import {
  BILLABLE_STATUS_LABELS,
  BILLABLE_STATUSES,
  showsProjectAndBillableFields,
  WORK_TYPE_LABELS,
  WORK_TYPES,
  type BillableStatus,
  type WorkType,
} from "@/lib/schedule";

const UNASSIGNED_PROJECT_VALUE = "__unassigned__";

export interface TimeEntryAllocationValues {
  workType: string;
  projectId?: number;
  billableStatus: string;
}

interface TimeEntryAllocationFieldsProps {
  idPrefix: string;
  projects: Project[];
  values: TimeEntryAllocationValues;
  onChange: (values: TimeEntryAllocationValues) => void;
}

export function TimeEntryAllocationFields({
  idPrefix,
  projects,
  values,
  onChange,
}: TimeEntryAllocationFieldsProps) {
  const showProjectAndBillable = showsProjectAndBillableFields(values.workType);
  const projectValue =
    typeof values.projectId === "number"
      ? values.projectId.toString()
      : UNASSIGNED_PROJECT_VALUE;

  return (
    <>
      <Field>
        <FieldLabel htmlFor={`${idPrefix}-work-type`}>Work type</FieldLabel>
        <Select
          value={values.workType}
          onValueChange={(workType) => {
            if (!showsProjectAndBillableFields(workType)) {
              onChange({
                workType,
                billableStatus: "unset",
              });
              return;
            }
            onChange({
              ...values,
              workType,
            });
          }}
        >
          <SelectTrigger id={`${idPrefix}-work-type`} className="w-full">
            <SelectValue placeholder="Worked" />
          </SelectTrigger>
          <SelectContent position="popper" align="start">
            {WORK_TYPES.map((workType) => (
              <SelectItem key={workType} value={workType}>
                {WORK_TYPE_LABELS[workType as WorkType]}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </Field>

      {showProjectAndBillable ? (
        <>
          <Field>
            <FieldLabel htmlFor={`${idPrefix}-project`}>Project</FieldLabel>
            <Select
              value={projectValue}
              onValueChange={(next) =>
                onChange({
                  ...values,
                  projectId:
                    next === UNASSIGNED_PROJECT_VALUE
                      ? undefined
                      : Number(next),
                })
              }
            >
              <SelectTrigger id={`${idPrefix}-project`} className="w-full">
                <SelectValue placeholder="None" />
              </SelectTrigger>
              <SelectContent position="popper" align="start">
                <SelectItem value={UNASSIGNED_PROJECT_VALUE}>None</SelectItem>
                {projects.map((project) => (
                  <SelectItem key={project.id} value={project.id.toString()}>
                    {project.name}
                    {project.archived ? " (archived)" : ""}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>

          <Field>
            <FieldLabel htmlFor={`${idPrefix}-billable`}>Billable</FieldLabel>
            <Select
              value={values.billableStatus}
              onValueChange={(billableStatus) =>
                onChange({
                  ...values,
                  billableStatus,
                })
              }
            >
              <SelectTrigger id={`${idPrefix}-billable`} className="w-full">
                <SelectValue placeholder="Unset" />
              </SelectTrigger>
              <SelectContent position="popper" align="start">
                {BILLABLE_STATUSES.map((status) => (
                  <SelectItem key={status} value={status}>
                    {BILLABLE_STATUS_LABELS[status as BillableStatus]}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>
        </>
      ) : null}
    </>
  );
}
