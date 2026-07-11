import { useGoogleAuthStatus } from "@/lib/api";
import type { GoogleAuthStatus } from "@/lib/api";
import type { IntegrationProviderId } from "./registry";

function googleAuthDescription(status: GoogleAuthStatus | undefined) {
  const keychain =
    "OAuth opens in your browser; tokens stay in the OS keychain.";
  if (!status) {
    return `Connect a Google account to import calendars. ${keychain}`;
  }
  if (status.mode === "local") {
    return `Auth: local / BYO credentials. Connect a Google account to import calendars. ${keychain}`;
  }
  const host = status.brokerBaseUrl
    ? (() => {
        try {
          return new URL(status.brokerBaseUrl).host;
        } catch {
          return status.brokerBaseUrl;
        }
      })()
    : "auth broker";
  return `Auth: broker (${host}). Connect a Google account to import calendars. ${keychain}`;
}

const staticDescriptions: Record<
  Exclude<IntegrationProviderId, "google">,
  string
> = {
  github:
    "Connect GitHub to pick repositories as evidence sources for AI gap-fill. OAuth and PAT tokens stay in the OS keychain.",
  slack:
    "Connect Slack to pick channels as read-only evidence sources for AI gap-fill. OAuth tokens stay in the OS keychain. Shiet never posts messages.",
};

export function AuthModeDescription({
  provider,
}: {
  provider: IntegrationProviderId;
}) {
  const googleAuthQuery = useGoogleAuthStatus();
  const description =
    provider === "google"
      ? googleAuthDescription(googleAuthQuery.data)
      : staticDescriptions[provider];

  return <>{description}</>;
}
