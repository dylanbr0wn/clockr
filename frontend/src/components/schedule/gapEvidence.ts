export function evidenceBadgeClass(provider: string) {
  switch (provider) {
    case "github":
      return "bg-slate-500/10 text-slate-700 dark:text-slate-300";
    case "slack":
      return "bg-violet-500/10 text-violet-700 dark:text-violet-300";
    case "bitbucket":
      return "bg-blue-500/10 text-blue-700 dark:text-blue-300";
    default:
      return "bg-muted text-muted-foreground";
  }
}

export function formatEvidenceLabel(provider: string, kind: string) {
  return `${provider || "unknown"} · ${kind || "activity"}`;
}
