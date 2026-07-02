import { formatDistanceToNow, parseISO } from "date-fns";
import type { SyncResult } from "@/lib/api/types";

export function formatRelativeTime(iso: string | undefined | null): string | null {
  if (!iso) {
    return null;
  }

  try {
    return formatDistanceToNow(parseISO(iso), { addSuffix: true });
  } catch {
    return null;
  }
}

export function formatSyncResultSummary(result: SyncResult): string {
  const parts: string[] = [];

  if (result.added > 0) {
    parts.push(`${result.added} added`);
  }
  if (result.updated > 0) {
    parts.push(`${result.updated} updated`);
  }
  if (result.removed > 0) {
    parts.push(`${result.removed} removed`);
  }
  if (result.flagged > 0) {
    parts.push(`${result.flagged} flagged`);
  }
  if (result.unchanged > 0 && parts.length === 0) {
    parts.push(`${result.unchanged} unchanged`);
  }

  return parts.length > 0 ? parts.join(", ") : "No changes";
}
