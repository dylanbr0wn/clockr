import { useEffect, useMemo } from "react";
import {
  Item,
  ItemActions,
  ItemContent,
  ItemGroup,
  ItemTitle,
} from "@/components/ui/item";
import { Toggle } from "@/components/ui/toggle";
import {
  useGitHubRepos,
  useIntegrationConnections,
  useSetGitHubRepoSelected,
} from "@/lib/api";
import { SettingBlock } from "../SettingBlock";
import { ScrollArea } from "../../ui/scroll-area";
import type { IntegrationConfigSlotProps } from "./types";

const PROVIDER_ID = "github";

export function GitHubEvidenceConfig({
  disabled = false,
  onBusyChange,
}: IntegrationConfigSlotProps) {
  const connectionsQuery = useIntegrationConnections();
  const reposQuery = useGitHubRepos();
  const setRepoSelected = useSetGitHubRepoSelected();

  const githubConnections = useMemo(
    () =>
      (connectionsQuery.data ?? []).filter(
        (connection) => connection.provider === PROVIDER_ID,
      ),
    [connectionsQuery.data],
  );

  const repos = reposQuery.data ?? [];

  const slotBusy = setRepoSelected.isPending;

  useEffect(() => {
    onBusyChange?.(slotBusy);
  }, [onBusyChange, slotBusy]);

  const isDisabled = disabled || slotBusy;

  return (
    <SettingBlock
      title="Repositories"
      description="Choose which repos to track as evidence for AI gap-fill. Tracked repos are used later when fetching commits and PRs."
    >
      {reposQuery.isLoading ? (
        <p className="text-sm text-muted-foreground">Loading repositories…</p>
      ) : repos.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          {githubConnections.length > 0
            ? "No repositories found. Try Refresh repos on the connection above."
            : "Connect a GitHub account to see repositories here."}
        </p>
      ) : (
        <ScrollArea className="max-h-64 h-64 overflow-auto rounded-md border px-2">
          <ItemGroup className="gap-2 my-2">
            {repos.map((repo) => (
              <Item key={repo.id} variant="outline">
                <ItemContent className="min-w-0">
                  <ItemTitle className="flex flex-wrap items-center gap-2">
                    <span className="truncate">{repo.fullName}</span>
                    {repo.private ? (
                      <span className="rounded-full bg-muted px-2 py-0.5 text-[10px] font-medium text-muted-foreground">
                        Private
                      </span>
                    ) : null}
                  </ItemTitle>
                </ItemContent>
                <ItemActions>
                  <Toggle
                    pressed={repo.selected}
                    variant="outline"
                    size="sm"
                    disabled={isDisabled}
                    aria-label={`Track ${repo.fullName} as evidence`}
                    onPressedChange={(pressed) => {
                      void setRepoSelected.mutateAsync({
                        repoID: repo.id,
                        selected: pressed,
                      });
                    }}
                  >
                    {repo.selected ? "Tracking" : "Track"}
                  </Toggle>
                </ItemActions>
              </Item>
            ))}
          </ItemGroup>
        </ScrollArea>
      )}
    </SettingBlock>
  );
}
