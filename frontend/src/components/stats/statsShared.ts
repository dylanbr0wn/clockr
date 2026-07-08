export function varianceToneClass(variance: number) {
  if (variance > 0) {
    return "text-emerald-700 dark:text-emerald-400";
  }

  if (variance < 0) {
    return "text-amber-700 dark:text-amber-400";
  }

  return "text-muted-foreground";
}

export function categoryBarWidth(minutes: number, maxMinutes: number) {
  if (maxMinutes <= 0) {
    return 0;
  }

  return Math.max(4, Math.round((minutes / maxMinutes) * 100));
}
