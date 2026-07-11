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
  useConnectSlack,
  useDisconnectSlack,
  useIntegrationConnections,
  useRefreshSlackChannels,
  useSetSlackChannelSelected,
  useSlackChannels,
  useSlackOAuthAvailable,
} from "@/lib/api";
import { SettingBlock } from "./SettingBlock";
import { ScrollArea } from "../ui/scroll-area";
import {
  AuthModeDescription,
  ConnectActions,
  ConnectionCard,
} from "./integrations";

export function SlackSettings() {
  const connectionsQuery = useIntegrationConnections();
  const channelsQuery = useSlackChannels();
  const oauthAvailableQuery = useSlackOAuthAvailable();
  const connectSlack = useConnectSlack();
  const disconnectSlack = useDisconnectSlack();
  const setChannelSelected = useSetSlackChannelSelected();
  const refreshChannels = useRefreshSlackChannels();

  const [connectError, setConnectError] = useState<string | null>(null);

  const slackConnections = useMemo(
    () =>
      (connectionsQuery.data ?? []).filter(
        (connection) => connection.provider === "slack",
      ),
    [connectionsQuery.data],
  );

  const channels = channelsQuery.data ?? [];

  const isBusy =
    connectSlack.isPending ||
    disconnectSlack.isPending ||
    setChannelSelected.isPending ||
    refreshChannels.isPending;

  const oauthAvailable = oauthAvailableQuery.isSuccess
    ? (oauthAvailableQuery.data ?? false)
    : false;

  const accountLabels = useMemo(() => {
    const labels = new Map<string, string>();
    for (const connection of slackConnections) {
      labels.set(connection.accountId, connection.accountLabel);
    }
    return labels;
  }, [slackConnections]);

  const handleOAuthConnect = async () => {
    setConnectError(null);
    try {
      await connectSlack.mutateAsync();
    } catch (error) {
      setConnectError(
        error instanceof Error
          ? error.message
          : "Unable to connect Slack workspace",
      );
    }
  };

  const handleDisconnect = async (accountID: string) => {
    setConnectError(null);
    try {
      await disconnectSlack.mutateAsync(accountID);
    } catch (error) {
      setConnectError(
        error instanceof Error
          ? error.message
          : "Unable to disconnect Slack workspace",
      );
    }
  };

  const handleRefresh = async (accountID: string) => {
    setConnectError(null);
    try {
      await refreshChannels.mutateAsync(accountID);
    } catch (error) {
      setConnectError(
        error instanceof Error
          ? error.message
          : "Unable to refresh Slack channels",
      );
    }
  };

  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <SettingBlock
        title="Slack"
        description={<AuthModeDescription provider="slack" />}
      >
        <ConnectActions
          provider="slack"
          oauthAvailable={oauthAvailable}
          onOAuthConnect={() => void handleOAuthConnect()}
          isConnecting={connectSlack.isPending}
          disabled={isBusy}
          connectError={connectError}
        />
      </SettingBlock>

      <SettingBlock
        title="Connected Workspaces"
        description="Connected Slack workspaces are used to list channels for evidence selection."
      >
        {slackConnections.length > 0 ? (
          <ItemGroup className="gap-2">
            {slackConnections.map((connection) => (
              <ConnectionCard
                key={connection.id}
                connection={connection}
                disabled={isBusy}
                onDisconnect={(accountID) => void handleDisconnect(accountID)}
                secondaryAction={{
                  label: "Refresh channels",
                  onClick: (accountID) => void handleRefresh(accountID),
                }}
              />
            ))}
          </ItemGroup>
        ) : (
          <p className="text-sm text-muted-foreground">
            No Slack workspace connected yet.
          </p>
        )}
      </SettingBlock>

      <SettingBlock
        title="Channels"
        description="Choose which channels to track as evidence for AI gap-fill. Tracked channels are used later when fetching message history."
      >
        {channelsQuery.isLoading ? (
          <p className="text-sm text-muted-foreground">Loading channels…</p>
        ) : channels.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            {slackConnections.length > 0
              ? "No channels found. Try Refresh channels on the connection above."
              : "Connect a Slack workspace to see channels here."}
          </p>
        ) : (
          <ScrollArea className="max-h-64 h-64 overflow-auto rounded-md border px-2">
            <ItemGroup className="gap-2 my-2">
              {channels.map((channel) => (
                <Item key={channel.id} variant="outline">
                  <ItemContent className="min-w-0">
                    <ItemTitle className="flex flex-wrap items-center gap-2">
                      <span className="truncate">#{channel.name}</span>
                      {accountLabels.get(channel.accountId) ? (
                        <span className="rounded-full bg-muted px-2 py-0.5 text-[10px] font-medium text-muted-foreground">
                          {accountLabels.get(channel.accountId)}
                        </span>
                      ) : null}
                      {channel.private ? (
                        <span className="rounded-full bg-muted px-2 py-0.5 text-[10px] font-medium text-muted-foreground">
                          Private
                        </span>
                      ) : null}
                    </ItemTitle>
                  </ItemContent>
                  <ItemActions>
                    <Toggle
                      pressed={channel.selected}
                      variant="outline"
                      size="sm"
                      disabled={isBusy}
                      aria-label={`Track #${channel.name} as evidence`}
                      onPressedChange={(pressed) => {
                        void setChannelSelected
                          .mutateAsync({
                            channelID: channel.id,
                            selected: pressed,
                          })
                          .catch((error) => {
                            setConnectError(
                              error instanceof Error
                                ? error.message
                                : "Unable to update channel selection",
                            );
                          });
                      }}
                    >
                      {channel.selected ? "Tracking" : "Track"}
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
