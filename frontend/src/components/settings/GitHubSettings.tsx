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
  useGitHubAuthMode,
  useGitHubOAuthAvailable,
  useGitHubRepos,
  useIntegrationConnections,
  useRefreshGitHubRepos,
  useSetGitHubRepoSelected,
} from "@/lib/api";
import { SettingBlock } from "./SettingBlock";
import { ScrollArea } from "../ui/scroll-area";

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
        description="Connect GitHub to pick repositories as evidence sources for AI gap-fill. OAuth and PAT tokens stay in the OS keychain."
      >
        <div className="space-y-3">
          {oauthAvailable ? (
            <Button
              type="button"
              disabled={isBusy}
              onClick={() => void handleOAuthConnect()}
            >
              {connectGitHub.isPending ? (
                <LoaderCircle className="size-4 animate-spin" />
              ) : (
                "Connect with GitHub"
              )}
            </Button>
          ) : null}

          <details
            className="rounded-md border border-border/70 p-3"
            open={authMode === "local"}
          >
            <summary className="cursor-pointer text-sm font-medium">
              Connect with a personal access token
            </summary>
            <p className="mt-1 text-xs text-muted-foreground">
              Local/advanced mode. The token is validated with GitHub and stored
              only in the OS keychain.
            </p>
            <div className="mt-3 grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end">
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
                      void handlePATConnect();
                    }
                  }}
                />
              </Field>
              <Button
                type="button"
                disabled={!pat.trim() || isBusy}
                onClick={() => void handlePATConnect()}
              >
                {connectGitHub.isPending ? (
                  <LoaderCircle className="size-4 animate-spin" />
                ) : (
                  "Connect"
                )}
              </Button>
            </div>
          </details>

          {connectError ? <FieldError>{connectError}</FieldError> : null}
        </div>
      </SettingBlock>
      <SettingBlock
        title="Connected Accounts"
        description="Connected GitHub accounts are used to fetch repositories, commits, and pull requests for AI gap-fill evidence."
      >
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
