import { useEffect, useMemo, useState } from "react";
import { ItemGroup } from "@/components/ui/item";
import {
  useConnectIntegration,
  useDisconnectIntegration,
  useIntegrationAuthStatus,
  useIntegrationConnections,
  useRefreshGitHubRepos,
  useRefreshSlackChannels,
  useRefreshBitbucketResources,
} from "@/lib/api";
import { SettingBlock } from "../SettingBlock";
import { AuthModeDescription } from "./AuthModeDescription";
import { ConnectActions } from "./ConnectActions";
import { ConnectionCard } from "./ConnectionCard";
import type { IntegrationProviderId } from "./registry";

type IntegrationConnectShellProps = {
  providerId: IntegrationProviderId;
  displayName: string;
  disabled?: boolean;
  onBusyChange?: (busy: boolean) => void;
};

const PROVIDER_LABELS: Record<
  IntegrationProviderId,
  { account: string; connected: string; empty: string; connectedDescription: string }
> = {
  google: {
    account: "Google account",
    connected: "Connected Google Accounts",
    empty: "No Google account connected yet.",
    connectedDescription: "Manage your connected Google accounts.",
  },
  github: {
    account: "GitHub account",
    connected: "Connected Accounts",
    empty: "No GitHub account connected yet.",
    connectedDescription:
      "Connected GitHub accounts are used to fetch repositories, commits, and pull requests for AI gap-fill evidence.",
  },
  slack: {
    account: "Slack workspace",
    connected: "Connected Workspaces",
    empty: "No Slack workspace connected yet.",
    connectedDescription:
      "Connected Slack workspaces are used to list channels for evidence selection.",
  },
  bitbucket: {
    account: "Bitbucket account",
    connected: "Connected Accounts",
    empty: "No Bitbucket account connected yet.",
    connectedDescription:
      "Connected Bitbucket accounts are used to list workspaces and repositories for evidence selection.",
  },
};

export function IntegrationConnectShell({
  providerId,
  displayName,
  disabled = false,
  onBusyChange,
}: IntegrationConnectShellProps) {
  const connectionsQuery = useIntegrationConnections();
  const connectIntegration = useConnectIntegration();
  const disconnectIntegration = useDisconnectIntegration();
  const refreshGitHubRepos = useRefreshGitHubRepos();
  const refreshSlackChannels = useRefreshSlackChannels();
  const refreshBitbucketResources = useRefreshBitbucketResources();

  const githubAuthQuery = useIntegrationAuthStatus("github", {
    enabled: providerId === "github",
  });
  const slackAuthQuery = useIntegrationAuthStatus("slack", {
    enabled: providerId === "slack",
  });
  const bitbucketAuthQuery = useIntegrationAuthStatus("bitbucket", {
    enabled: providerId === "bitbucket",
  });

  const [accountEmail, setAccountEmail] = useState("");
  const [pat, setPat] = useState("");
  const [connectError, setConnectError] = useState<string | null>(null);

  const providerConnections = useMemo(
    () =>
      (connectionsQuery.data ?? []).filter(
        (connection) => connection.provider === providerId,
      ),
    [connectionsQuery.data, providerId],
  );

  const connectBusy =
    connectIntegration.isPending ||
    disconnectIntegration.isPending ||
    (providerId === "github" && refreshGitHubRepos.isPending) ||
    (providerId === "slack" && refreshSlackChannels.isPending) ||
    (providerId === "bitbucket" && refreshBitbucketResources.isPending);

  useEffect(() => {
    onBusyChange?.(connectBusy);
  }, [connectBusy, onBusyChange]);

  const isDisabled = disabled || connectBusy;
  const labels = PROVIDER_LABELS[providerId];

  const handleConnectError = (error: unknown, action: string) => {
    setConnectError(
      error instanceof Error
        ? error.message
        : `Unable to ${action} ${labels.account}`,
    );
  };

  const handleGoogleConnect = async (accountID: string, accountLabel: string) => {
    setConnectError(null);
    try {
      await connectIntegration.mutateAsync({
        provider: "google",
        accountId: accountID,
        accountLabel,
      });
      setAccountEmail("");
    } catch (error) {
      handleConnectError(error, "connect");
    }
  };

  const handleGitHubOAuthConnect = async () => {
    setConnectError(null);
    try {
      await connectIntegration.mutateAsync({
        provider: "github",
        pat: "",
      });
    } catch (error) {
      handleConnectError(error, "connect");
    }
  };

  const handleGitHubPATConnect = async () => {
    const token = pat.trim();
    if (!token) {
      return;
    }

    setConnectError(null);
    try {
      await connectIntegration.mutateAsync({
        provider: "github",
        pat: token,
      });
      setPat("");
    } catch (error) {
      handleConnectError(error, "connect");
    }
  };

  const handleDisconnect = async (accountID: string) => {
    setConnectError(null);
    try {
      await disconnectIntegration.mutateAsync({
        provider: providerId,
        accountID,
      });
    } catch (error) {
      handleConnectError(error, "disconnect");
    }
  };

  const handleReconnect = async (accountID: string, accountLabel: string) => {
    setConnectError(null);
    try {
      await connectIntegration.mutateAsync({
        provider: providerId,
        accountId: accountID,
        accountLabel,
      });
    } catch (error) {
      handleConnectError(error, "reconnect");
    }
  };

  const handleRefreshRepos = async (accountID: string) => {
    setConnectError(null);
    try {
      await refreshGitHubRepos.mutateAsync(accountID);
    } catch (error) {
      setConnectError(
        error instanceof Error
          ? error.message
          : "Unable to refresh GitHub repositories",
      );
    }
  };

  const handleSlackOAuthConnect = async () => {
    setConnectError(null);
    try {
      await connectIntegration.mutateAsync({
        provider: "slack",
      });
    } catch (error) {
      handleConnectError(error, "connect");
    }
  };

  const handleRefreshChannels = async (accountID: string) => {
    setConnectError(null);
    try {
      await refreshSlackChannels.mutateAsync(accountID);
    } catch (error) {
      setConnectError(
        error instanceof Error
          ? error.message
          : "Unable to refresh Slack channels",
      );
    }
  };

  const handleBitbucketOAuthConnect = async () => {
    setConnectError(null);
    try {
      await connectIntegration.mutateAsync({
        provider: "bitbucket",
      });
    } catch (error) {
      handleConnectError(error, "connect");
    }
  };

  const handleRefreshBitbucketResources = async (accountID: string) => {
    setConnectError(null);
    try {
      await refreshBitbucketResources.mutateAsync(accountID);
    } catch (error) {
      setConnectError(
        error instanceof Error
          ? error.message
          : "Unable to refresh Bitbucket resources",
      );
    }
  };

  if (
    providerId !== "google" &&
    providerId !== "github" &&
    providerId !== "slack" &&
    providerId !== "bitbucket"
  ) {
    return null;
  }

  const githubAuthMode = githubAuthQuery.data?.mode ?? "broker";
  const githubOauthAvailable = githubAuthQuery.data?.oauthAvailable ?? true;
  const slackOauthAvailable = slackAuthQuery.data?.oauthAvailable ?? false;
  const bitbucketOauthAvailable = bitbucketAuthQuery.data?.oauthAvailable ?? false;

  return (
    <>
      <SettingBlock
        title={displayName}
        description={<AuthModeDescription provider={providerId} />}
      >
        {providerId === "google" ? (
          <ConnectActions
            provider="google"
            accountEmail={accountEmail}
            onAccountEmailChange={setAccountEmail}
            onConnect={() => {
              const email = accountEmail.trim();
              if (!email) {
                return;
              }
              void handleGoogleConnect(email, email);
            }}
            isConnecting={connectIntegration.isPending}
            disabled={isDisabled}
            connectError={connectError}
          />
        ) : providerId === "github" ? (
          <ConnectActions
            provider="github"
            oauthAvailable={githubOauthAvailable}
            authMode={githubAuthMode}
            pat={pat}
            onPatChange={setPat}
            onOAuthConnect={() => void handleGitHubOAuthConnect()}
            onPatConnect={() => void handleGitHubPATConnect()}
            isConnecting={connectIntegration.isPending}
            disabled={isDisabled}
            connectError={connectError}
          />
        ) : providerId === "slack" ? (
          <ConnectActions
            provider="slack"
            oauthAvailable={slackOauthAvailable}
            onOAuthConnect={() => void handleSlackOAuthConnect()}
            isConnecting={connectIntegration.isPending}
            disabled={isDisabled}
            connectError={connectError}
          />
        ) : (
          <ConnectActions
            provider="bitbucket"
            oauthAvailable={bitbucketOauthAvailable}
            onOAuthConnect={() => void handleBitbucketOAuthConnect()}
            isConnecting={connectIntegration.isPending}
            disabled={isDisabled}
            connectError={connectError}
          />
        )}
      </SettingBlock>
      <SettingBlock
        title={labels.connected}
        description={labels.connectedDescription}
      >
        {providerConnections.length > 0 ? (
          <ItemGroup className="gap-2">
            {providerConnections.map((connection) => (
              <ConnectionCard
                key={connection.id}
                connection={connection}
                disabled={isDisabled}
                onDisconnect={(accountID) => void handleDisconnect(accountID)}
                onReconnect={
                  providerId === "google"
                    ? (accountID, accountLabel) =>
                        void handleReconnect(accountID, accountLabel)
                    : undefined
                }
                secondaryAction={
                  providerId === "github"
                    ? {
                        label: "Refresh repos",
                        onClick: (accountID) => void handleRefreshRepos(accountID),
                      }
                    : providerId === "slack"
                      ? {
                          label: "Refresh channels",
                          onClick: (accountID) => void handleRefreshChannels(accountID),
                        }
                      : providerId === "bitbucket"
                        ? {
                            label: "Refresh resources",
                            onClick: (accountID) =>
                              void handleRefreshBitbucketResources(accountID),
                          }
                        : undefined
                }
              />
            ))}
          </ItemGroup>
        ) : (
          <p className="text-sm text-muted-foreground">{labels.empty}</p>
        )}
      </SettingBlock>
    </>
  );
}
