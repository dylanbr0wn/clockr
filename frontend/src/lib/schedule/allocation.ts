export const WORK_TYPES = [
  "worked",
  "paid_leave",
  "unpaid_leave",
  "holiday",
  "break",
  "adjustment",
] as const;

export type WorkType = (typeof WORK_TYPES)[number];

export const DEFAULT_WORK_TYPE: WorkType = "worked";

export const BILLABLE_STATUSES = [
  "unset",
  "billable",
  "non_billable",
] as const;

export type BillableStatus = (typeof BILLABLE_STATUSES)[number];

export const DEFAULT_BILLABLE_STATUS: BillableStatus = "unset";

const HIDDEN_ALLOCATION_WORK_TYPES = new Set<string>([
  "paid_leave",
  "unpaid_leave",
  "holiday",
  "break",
]);

/** Leave/break/holiday: hide project + billable in schedule editors. */
export function showsProjectAndBillableFields(workType: string): boolean {
  return !HIDDEN_ALLOCATION_WORK_TYPES.has(workType);
}

export const WORK_TYPE_LABELS: Record<WorkType, string> = {
  worked: "Worked",
  paid_leave: "Paid leave",
  unpaid_leave: "Unpaid leave",
  holiday: "Holiday",
  break: "Break",
  adjustment: "Adjustment",
};

export const BILLABLE_STATUS_LABELS: Record<BillableStatus, string> = {
  unset: "Unset",
  billable: "Billable",
  non_billable: "Non-billable",
};
