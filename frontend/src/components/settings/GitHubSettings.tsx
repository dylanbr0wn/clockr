import {
  AlertCircle,
  CheckCircle2,
  LoaderCircle,
  LogOut,
  RefreshCw,
} from "lucide-react";
import { useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Field,
  FieldError,
  FieldLabel,
} from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import {
  Item,
  ItemActions,
  ItemContent,
  ItemDescription,
  ItemGroup,
  ItemTitle,
} from "@/components/ui/item";
import { Toggle } from "@/components/ui/toggle";
import {
  useConnectGitHub,
  useDisconnectGitHub,
  useGitHubRepos,
  useIntegrationConnections,
  useRefreshGitHubRepos,
  useSetGitHubRepoSelected,
} from "@/lib/api";
import { SettingBlock } from "./SettingBlock";

function connectionStatusLabel(status: string) {
  switch (status) {
    case "connected":
      return "Connected";
    case "needs_reauth":
      return "Needs re-auth";
    case "disconnected":
      return "Disconnected";
    default:
      return status;
  }
}

function ConnectionStatusBadge({ status }: { status: string }) {
  if (status === "connected") {
    return (
      <span className="inline-flex items-center gap-1 rounded-full bg-emerald-500/10 px-2 py-0.5 text-[10px] font-medium text-emerald-700 dark:text-emerald-300">
        <CheckCircle2 className="size-3" />
        Connected
      </span>
    );
  }

  if (status === "needs_reauth") {
    return (
      <span className="inline-flex items-center gap-1 rounded-full bg-amber-500/10 px-2 py-0.5 text-[10px] font-medium text-amber-700 dark:text-amber-300">
        <AlertCircle className="size-3" />
        Needs re-auth
      </span>
    );
  }

  return (
    <span className="rounded-full bg-muted px-2 py-0.5 text-[10px] font-medium text-muted-foreground">
      {connectionStatusLabel(status)}
    </span>
  );
}

export function GitHubSettings() {
  const connectionsQuery = useIntegrationConnections();
  const reposQuery = useGitHubRepos();
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

  const handleConnect = async () => {
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
        description="Connect with a personal access token to pick repositories as evidence sources for AI gap-fill. Tokens stay in the OS keychain."
      >
        <div className="space-y-3">
          {githubConnections.length > 0 ? (
            <ItemGroup className="gap-2">
              {githubConnections.map((connection) => (
                <Item key={connection.id} variant="outline">
                  <ItemContent className="min-w-0">
                    <ItemTitle className="flex flex-wrap items-center gap-2">
                      <span className="truncate">{connection.accountLabel}</span>
                      <ConnectionStatusBadge status={connection.status} />
                    </ItemTitle>
                    <ItemDescription className="truncate">
                      {connection.accountId}
                    </ItemDescription>
                  </ItemContent>
                  <ItemActions>
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      disabled={isBusy}
                      onClick={() => void handleRefresh(connection.accountId)}
                    >
                      <RefreshCw className="size-4" />
                      Refresh repos
                    </Button>
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      disabled={isBusy}
                      onClick={() => void handleDisconnect(connection.accountId)}
                    >
                      <LogOut className="size-4" />
                      Disconnect
                    </Button>
                  </ItemActions>
                </Item>
              ))}
            </ItemGroup>
          ) : (
            <p className="text-sm text-muted-foreground">
              No GitHub account connected yet.
            </p>
          )}

          <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end">
            <Field>
              <FieldLabel htmlFor="github-pat">
                Personal access token
              </FieldLabel>
              <Input
                id="github-pat"
                type="password"
                autoComplete="off"
                value={pat}
                placeholder="ghp_… or github_pat_…"
                onChange={(event) => setPat(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Enter") {
                    void handleConnect();
                  }
                }}
              />
            </Field>
            <Button
              type="button"
              disabled={!pat.trim() || isBusy}
              onClick={() => void handleConnect()}
            >
              {connectGitHub.isPending ? (
                <LoaderCircle className="size-4 animate-spin" />
              ) : (
                "Connect"
              )}
            </Button>
          </div>

          {connectError ? <FieldError>{connectError}</FieldError> : null}
        </div>
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
          <ItemGroup className="max-h-64 gap-2 overflow-y-auto pr-1">
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
        )}
      </SettingBlock>
    </div>
  );
}
