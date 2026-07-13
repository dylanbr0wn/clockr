import { useIntegrationAuthStatus } from "@/lib/api";
import type { IntegrationAuthStatus } from "@/lib/api";
import type { IntegrationProviderId } from "./registry";

function brokerHost(status: IntegrationAuthStatus) {
  if (!status.brokerBaseUrl) {
    return "auth broker";
  }
  try {
    return new URL(status.brokerBaseUrl).host;
  } catch {
    return status.brokerBaseUrl;
  }
}

function googleAuthDescription(status: IntegrationAuthStatus | undefined) {
  const keychain =
    "OAuth opens in your browser; tokens stay in the OS keychain.";
  if (!status) {
    return `Connect a Google account to import calendars. ${keychain}`;
  }
  if (status.mode === "local") {
    return `Auth: local / BYO credentials. Connect a Google account to import calendars. ${keychain}`;
  }
  return `Auth: broker (${brokerHost(status)}). Connect a Google account to import calendars. ${keychain}`;
}

function githubAuthDescription(status: IntegrationAuthStatus | undefined) {
  const keychain =
    "OAuth and PAT tokens stay in the OS keychain.";
  const purpose =
    "Connect GitHub to pick repositories as evidence sources for AI gap-fill.";
  if (!status) {
    return `${purpose} ${keychain}`;
  }
  if (status.mode === "local") {
    return `Auth: local / BYO credentials. ${purpose} ${keychain}`;
  }
  return `Auth: broker (${brokerHost(status)}). ${purpose} ${keychain}`;
}

function slackAuthDescription(status: IntegrationAuthStatus | undefined) {
  const keychain =
    "OAuth tokens stay in the OS keychain. Shiet never posts messages.";
  const purpose =
    "Connect Slack to pick channels as read-only evidence sources for AI gap-fill.";
  if (!status) {
    return `${purpose} ${keychain}`;
  }
  if (status.mode === "local") {
    return `Auth: local / BYO credentials. ${purpose} ${keychain}`;
  }
  return `Auth: broker (${brokerHost(status)}). ${purpose} ${keychain}`;
}

function bitbucketAuthDescription(status: IntegrationAuthStatus | undefined) {
  const keychain = "OAuth tokens stay in the OS keychain.";
  const purpose =
    "Connect Bitbucket to pick workspaces and repositories as read-only evidence sources for AI gap-fill.";
  if (!status) {
    return `${purpose} ${keychain}`;
  }
  if (status.mode === "local") {
    return `Auth: local / BYO credentials. ${purpose} ${keychain}`;
  }
  return `Auth: broker (${brokerHost(status)}). ${purpose} ${keychain}`;
}

export function AuthModeDescription({
  provider,
}: {
  provider: IntegrationProviderId;
}) {
  const googleAuthQuery = useIntegrationAuthStatus("google", {
    enabled: provider === "google",
  });
  const githubAuthQuery = useIntegrationAuthStatus("github", {
    enabled: provider === "github",
  });
  const slackAuthQuery = useIntegrationAuthStatus("slack", {
    enabled: provider === "slack",
  });
  const bitbucketAuthQuery = useIntegrationAuthStatus("bitbucket", {
    enabled: provider === "bitbucket",
  });

  const description =
    provider === "google"
      ? googleAuthDescription(googleAuthQuery.data)
      : provider === "github"
        ? githubAuthDescription(githubAuthQuery.data)
        : provider === "slack"
          ? slackAuthDescription(slackAuthQuery.data)
          : bitbucketAuthDescription(bitbucketAuthQuery.data);

  return <>{description}</>;
}
