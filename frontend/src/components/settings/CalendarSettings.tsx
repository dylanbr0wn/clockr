import { useMemo, useState } from "react";
import {
  Field,
  FieldLabel,
} from "@/components/ui/field";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Item,
  ItemActions,
  ItemContent,
  ItemGroup,
  ItemTitle,
} from "@/components/ui/item";
import { Toggle } from "@/components/ui/toggle";
import {
  useCalendars,
  useCategories,
  useConnectGoogle,
  useDisconnectGoogle,
  useIntegrationConnections,
  useSetCalendarDefaultCategory,
  useSetCalendarSelected,
} from "@/lib/api";
import { SettingBlock } from "./SettingBlock";
import {
  AuthModeDescription,
  ConnectActions,
  ConnectionCard,
} from "./integrations";

const NONE_CATEGORY = "__none__";

export function CalendarSettings() {
  const connectionsQuery = useIntegrationConnections();
  const calendarsQuery = useCalendars();
  const categoriesQuery = useCategories();
  const connectGoogle = useConnectGoogle();
  const disconnectGoogle = useDisconnectGoogle();
  const setCalendarSelected = useSetCalendarSelected();
  const setCalendarDefaultCategory = useSetCalendarDefaultCategory();

  const [accountEmail, setAccountEmail] = useState("");
  const [connectError, setConnectError] = useState<string | null>(null);

  const googleConnections = useMemo(
    () =>
      (connectionsQuery.data ?? []).filter(
        (connection) => connection.provider === "google",
      ),
    [connectionsQuery.data],
  );

  const googleCalendars = useMemo(
    () =>
      (calendarsQuery.data ?? []).filter(
        (calendar) => calendar.provider === "google",
      ),
    [calendarsQuery.data],
  );

  const categories = categoriesQuery.data ?? [];

  const isBusy =
    connectGoogle.isPending ||
    disconnectGoogle.isPending ||
    setCalendarSelected.isPending ||
    setCalendarDefaultCategory.isPending;

  const handleConnect = async () => {
    const email = accountEmail.trim();
    if (!email) {
      return;
    }

    setConnectError(null);
    try {
      await connectGoogle.mutateAsync({
        accountID: email,
        accountLabel: email,
      });
      setAccountEmail("");
    } catch (error) {
      setConnectError(
        error instanceof Error ? error.message : "Unable to connect Google account",
      );
    }
  };

  const handleDisconnect = async (accountID: string) => {
    setConnectError(null);
    try {
      await disconnectGoogle.mutateAsync(accountID);
    } catch (error) {
      setConnectError(
        error instanceof Error
          ? error.message
          : "Unable to disconnect Google account",
      );
    }
  };

  const handleReconnect = async (accountID: string, accountLabel: string) => {
    setConnectError(null);
    try {
      await connectGoogle.mutateAsync({ accountID, accountLabel });
    } catch (error) {
      setConnectError(
        error instanceof Error ? error.message : "Unable to reconnect Google account",
      );
    }
  };

  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <SettingBlock
        title="Google Calendar"
        description={<AuthModeDescription provider="google" />}
      >
        <ConnectActions
          provider="google"
          accountEmail={accountEmail}
          onAccountEmailChange={setAccountEmail}
          onConnect={() => void handleConnect()}
          isConnecting={connectGoogle.isPending}
          disabled={isBusy}
          connectError={connectError}
        />
      </SettingBlock>
      <SettingBlock
        title="Connected Google Accounts"
        description="Manage your connected Google accounts."
      >
        {googleConnections.length > 0 ? (
          <ItemGroup className="gap-2">
            {googleConnections.map((connection) => (
              <ConnectionCard
                key={connection.id}
                connection={connection}
                disabled={isBusy}
                onDisconnect={(accountID) => void handleDisconnect(accountID)}
                onReconnect={(accountID, accountLabel) =>
                  void handleReconnect(accountID, accountLabel)
                }
              />
            ))}
          </ItemGroup>
        ) : (
          <p className="text-sm text-muted-foreground">
            No Google account connected yet.
          </p>
        )}
      </SettingBlock>
      <SettingBlock
        title="Calendars"
        description="Choose which calendars to import. Primary is selected by default on first connect."
      >
        {calendarsQuery.isLoading ? (
          <p className="text-sm text-muted-foreground">Loading calendars…</p>
        ) : googleCalendars.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            {googleConnections.length > 0
              ? "Connect syncs your calendar list. Run Sync on the schedule view if calendars are missing."
              : "Connect a Google account to see calendars here."}
          </p>
        ) : (
          <ItemGroup className="gap-2">
            {googleCalendars.map((calendar) => (
              <Item
                key={calendar.id}
                variant="outline"
                className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto_minmax(0,180px)] sm:items-center"
              >
                <ItemContent className="min-w-0">
                  <ItemTitle className="flex flex-wrap items-center gap-2">
                    <span className="truncate">{calendar.name}</span>
                    {calendar.isPrimary ? (
                      <span className="rounded-full bg-primary/10 px-2 py-0.5 text-[10px] font-medium text-primary">
                        Primary
                      </span>
                    ) : null}
                  </ItemTitle>
                </ItemContent>

                <ItemActions>
                  <Toggle
                    pressed={calendar.selected}
                    variant="outline"
                    size="sm"
                    disabled={isBusy}
                    aria-label={`Import ${calendar.name}`}
                    onPressedChange={(pressed) => {
                      void setCalendarSelected.mutateAsync({
                        calendarID: calendar.id,
                        selected: pressed,
                      });
                    }}
                  >
                    {calendar.selected ? "Importing" : "Import"}
                  </Toggle>
                </ItemActions>

                <ItemContent className="min-w-0">
                  <Field>
                    <FieldLabel
                      htmlFor={`calendar-category-${calendar.id}`}
                      className="sr-only"
                    >
                      Default category for {calendar.name}
                    </FieldLabel>
                    <Select
                      value={
                        calendar.defaultCategoryId
                          ? String(calendar.defaultCategoryId)
                          : NONE_CATEGORY
                      }
                      onValueChange={(value) => {
                        void setCalendarDefaultCategory.mutateAsync({
                          calendarID: calendar.id,
                          categoryID:
                            value === NONE_CATEGORY ? null : Number(value),
                        });
                      }}
                      disabled={isBusy}
                    >
                      <SelectTrigger
                        id={`calendar-category-${calendar.id}`}
                        className="w-full bg-background"
                      >
                        <SelectValue placeholder="Default category" />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value={NONE_CATEGORY}>No default</SelectItem>
                        {categories.map((category) => (
                          <SelectItem
                            key={category.id}
                            value={String(category.id)}
                          >
                            {category.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </Field>
                </ItemContent>
              </Item>
            ))}
          </ItemGroup>
        )}
      </SettingBlock>
    </div>
  );
}
