export function kindBadgeClass(kind: string) {
  switch (kind) {
    case "deleted_categorized":
      return "bg-destructive/10 text-destructive";
    case "new_in_gap":
      return "bg-amber-500/10 text-amber-700 dark:text-amber-300";
    case "title_changed":
      return "bg-blue-500/10 text-blue-700 dark:text-blue-300";
    case "tentative":
      return "bg-violet-500/10 text-violet-700 dark:text-violet-300";
    case "all_day":
      return "bg-slate-500/10 text-slate-700 dark:text-slate-300";
    default:
      return "bg-muted text-muted-foreground";
  }
}
