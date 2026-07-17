/** User-facing label for ExpectedTime resolver source / exception kind. */
export function formatExpectedTimeSourceLabel(
  source: string,
  exceptionKind?: string,
): string {
  if (source === "weekday") {
    return "Weekday template";
  }
  if (source !== "exception") {
    return "";
  }
  switch (exceptionKind) {
    case "holiday":
      return "Holiday";
    case "leave":
      return "Leave";
    case "changed_hours":
      return "Changed hours";
    default:
      return "";
  }
}
