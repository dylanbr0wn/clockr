import type { ComponentType } from "react";
import { CalendarSettings } from "../CalendarSettings";
import { GitHubSettings } from "../GitHubSettings";
import { SlackSettings } from "../SlackSettings";

export type IntegrationKind = "calendar_source" | "activity_evidence";

export type IntegrationProviderId = "google" | "github" | "slack";

export type IntegrationCatalogEntry = {
  id: IntegrationProviderId;
  displayName: string;
  kind: IntegrationKind;
  Panel: ComponentType;
};

export const integrationKindLabels: Record<IntegrationKind, string> = {
  calendar_source: "Calendar sources",
  activity_evidence: "Activity evidence",
};

export const integrationRegistry: IntegrationCatalogEntry[] = [
  {
    id: "google",
    displayName: "Google Calendar",
    kind: "calendar_source",
    Panel: CalendarSettings,
  },
  {
    id: "github",
    displayName: "GitHub",
    kind: "activity_evidence",
    Panel: GitHubSettings,
  },
  {
    id: "slack",
    displayName: "Slack",
    kind: "activity_evidence",
    Panel: SlackSettings,
  },
];

export function getIntegrationEntry(
  providerId: string,
): IntegrationCatalogEntry | undefined {
  return integrationRegistry.find((entry) => entry.id === providerId);
}

export function groupIntegrationsByKind(): Array<{
  kind: IntegrationKind;
  label: string;
  entries: IntegrationCatalogEntry[];
}> {
  const kinds: IntegrationKind[] = ["calendar_source", "activity_evidence"];
  return kinds.map((kind) => ({
    kind,
    label: integrationKindLabels[kind],
    entries: integrationRegistry.filter((entry) => entry.kind === kind),
  }));
}
