import { useMemo, useState } from "react";
import {
  Item,
  ItemActions,
  ItemContent,
  ItemGroup,
  ItemTitle,
} from "@/components/ui/item";
import { Toggle } from "@/components/ui/toggle";
import {
  useConnectGitHub,
  useDisconnectGitHub,
  useGitHubAuthMode,
  useGitHubOAuthAvailable,
  useGitHubRepos,
  useIntegrationConnections,
  useRefreshGitHubRepos,
  useSetGitHubRepoSelected,
} from "@/lib/api";
import { SettingBlock } from "./SettingBlock";
import { ScrollArea } from "../ui/scroll-area";
import {
  AuthModeDescription,
  ConnectActions,
  ConnectionCard,
} from "./integrations";

export function GitHubSettings() {
  const connectionsQuery = useIntegrationConnections();
  const reposQuery = useGitHubRepos();
  const authModeQuery = useGitHubAuthMode();
  const oauthAvailableQuery = useGitHubOAuthAvailable();
  const connectGitHub = useConnectGitHub();
  const disconnectGitHub = useDisconnectGitHub();
  const setRepoSelected = useSetGitHubRepoSelected();
  const refreshRepos = useRefreshGitHubRepos();

  const [pat, setPat] = useState("");
  const [connectError, setConnectError] = useState<string | null>(null);

  const githubConnections = useMemo(
    () =>
      (connectionsQuery.data ?? []).filter(
        (connection) => connection.provider === "github",
      ),
    [connectionsQuery.data],
  );

  const repos = reposQuery.data ?? [];

  const isBusy =
    connectGitHub.isPending ||
    disconnectGitHub.isPending ||
    setRepoSelected.isPending ||
    refreshRepos.isPending;

  const authMode = authModeQuery.data ?? "broker";
  const oauthAvailable = oauthAvailableQuery.data ?? true;

  const handlePATConnect = async () => {
    const token = pat.trim();
    if (!token) {
      return;
    }

    setConnectError(null);
    try {
      await connectGitHub.mutateAsync(token);
      setPat("");
    } catch (error) {
      setConnectError(
        error instanceof Error
          ? error.message
          : "Unable to connect GitHub account",
      );
    }
  };

  const handleOAuthConnect = async () => {
    setConnectError(null);
    try {
      await connectGitHub.mutateAsync("");
    } catch (error) {
      setConnectError(
        error instanceof Error
          ? error.message
          : "Unable to connect GitHub account",
      );
    }
  };

  const handleDisconnect = async (accountID: string) => {
    setConnectError(null);
    try {
      await disconnectGitHub.mutateAsync(accountID);
    } catch (error) {
      setConnectError(
        error instanceof Error
          ? error.message
          : "Unable to disconnect GitHub account",
      );
    }
  };

  const handleRefresh = async (accountID: string) => {
    setConnectError(null);
    try {
      await refreshRepos.mutateAsync(accountID);
    } catch (error) {
      setConnectError(
        error instanceof Error
          ? error.message
          : "Unable to refresh GitHub repositories",
      );
    }
  };

  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <SettingBlock
        title="GitHub"
        description={<AuthModeDescription provider="github" />}
      >
        <ConnectActions
          provider="github"
          oauthAvailable={oauthAvailable}
          authMode={authMode}
          pat={pat}
          onPatChange={setPat}
          onOAuthConnect={() => void handleOAuthConnect()}
          onPatConnect={() => void handlePATConnect()}
          isConnecting={connectGitHub.isPending}
          disabled={isBusy}
          connectError={connectError}
        />
      </SettingBlock>
      <SettingBlock
        title="Connected Accounts"
        description="Connected GitHub accounts are used to fetch repositories, commits, and pull requests for AI gap-fill evidence."
      >
        {githubConnections.length > 0 ? (
          <ItemGroup className="gap-2">
            {githubConnections.map((connection) => (
              <ConnectionCard
                key={connection.id}
                connection={connection}
                disabled={isBusy}
                onDisconnect={(accountID) => void handleDisconnect(accountID)}
                secondaryAction={{
                  label: "Refresh repos",
                  onClick: (accountID) => void handleRefresh(accountID),
                }}
              />
            ))}
          </ItemGroup>
        ) : (
          <p className="text-sm text-muted-foreground">
            No GitHub account connected yet.
          </p>
        )}
      </SettingBlock>

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
                          disabled={isBusy}
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
    </div>
  );
}
