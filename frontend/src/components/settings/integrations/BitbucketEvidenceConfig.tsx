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
  useBitbucketRepos,
  useBitbucketWorkspaces,
  useIntegrationConnections,
  useSetBitbucketRepoSelected,
  useSetBitbucketWorkspaceSelected,
} from "@/lib/api";
import { SettingBlock } from "../SettingBlock";
import { ScrollArea } from "../../ui/scroll-area";
import type { IntegrationConfigSlotProps } from "./types";

const PROVIDER_ID = "bitbucket";

export function BitbucketEvidenceConfig({
  disabled = false,
  onBusyChange,
}: IntegrationConfigSlotProps) {
  const connectionsQuery = useIntegrationConnections();
  const workspacesQuery = useBitbucketWorkspaces();
  const reposQuery = useBitbucketRepos();
  const setWorkspaceSelected = useSetBitbucketWorkspaceSelected();
  const setRepoSelected = useSetBitbucketRepoSelected();

  const bitbucketConnections = useMemo(
    () =>
      (connectionsQuery.data ?? []).filter(
        (connection) => connection.provider === PROVIDER_ID,
      ),
    [connectionsQuery.data],
  );

  const workspaces = workspacesQuery.data ?? [];
  const repos = reposQuery.data ?? [];

  const slotBusy =
    setWorkspaceSelected.isPending || setRepoSelected.isPending;

  useEffect(() => {
    onBusyChange?.(slotBusy);
  }, [onBusyChange, slotBusy]);

  const isDisabled = disabled || slotBusy;

  const reposByWorkspace = useMemo(() => {
    const grouped = new Map<string, typeof repos>();
    for (const repo of repos) {
      const list = grouped.get(repo.workspaceUuid) ?? [];
      list.push(repo);
      grouped.set(repo.workspaceUuid, list);
    }
    return grouped;
  }, [repos]);

  return (
    <>
      <SettingBlock
        title="Workspaces"
        description="Choose which Bitbucket workspaces to include. Repos under tracked workspaces can be selected below."
      >
        {workspacesQuery.isLoading ? (
          <p className="text-sm text-muted-foreground">Loading workspaces…</p>
        ) : workspaces.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            {bitbucketConnections.length > 0
              ? "No workspaces found. Try Refresh resources on the connection above."
              : "Connect a Bitbucket account to see workspaces here."}
          </p>
        ) : (
          <ScrollArea className="max-h-48 h-48 overflow-auto rounded-md border px-2">
            <ItemGroup className="gap-2 my-2">
              {workspaces.map((workspace) => (
                <Item key={workspace.id} variant="outline">
                  <ItemContent className="min-w-0">
                    <ItemTitle className="truncate">{workspace.name}</ItemTitle>
                    <p className="text-xs text-muted-foreground">{workspace.slug}</p>
                  </ItemContent>
                  <ItemActions>
                    <Toggle
                      pressed={workspace.selected}
                      variant="outline"
                      size="sm"
                      disabled={isDisabled}
                      onPressedChange={(pressed) => {
                        void setWorkspaceSelected.mutateAsync({
                          workspaceID: workspace.id,
                          selected: pressed,
                        });
                      }}
                    >
                      {workspace.selected ? "Tracking" : "Track"}
                    </Toggle>
                  </ItemActions>
                </Item>
              ))}
            </ItemGroup>
          </ScrollArea>
        )}
      </SettingBlock>

      <SettingBlock
        title="Repositories"
        description="Choose which repos to track as evidence for AI gap-fill. Tracked repos are used later when fetching commits."
      >
        {reposQuery.isLoading ? (
          <p className="text-sm text-muted-foreground">Loading repositories…</p>
        ) : repos.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            {bitbucketConnections.length > 0
              ? "No repositories found. Try Refresh resources on the connection above."
              : "Connect a Bitbucket account to see repositories here."}
          </p>
        ) : (
          <ScrollArea className="max-h-64 h-64 overflow-auto rounded-md border px-2">
            <ItemGroup className="gap-4 my-2">
              {workspaces.map((workspace) => {
                const workspaceRepos = reposByWorkspace.get(workspace.externalId) ?? [];
                if (workspaceRepos.length === 0) {
                  return null;
                }
                return (
                  <div key={workspace.id} className="space-y-2">
                    <p className="px-1 text-xs font-medium text-muted-foreground">
                      {workspace.name}
                    </p>
                    <ItemGroup className="gap-2">
                      {workspaceRepos.map((repo) => (
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
                              disabled={isDisabled || !workspace.selected}
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
                  </div>
                );
              })}
            </ItemGroup>
          </ScrollArea>
        )}
      </SettingBlock>
    </>
  );
}
