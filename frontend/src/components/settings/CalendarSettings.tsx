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
  ItemDescription,
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

const NONE_CATEGORY = "__none__";

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
        description="Connect a Google account to import calendars. OAuth opens in your browser; tokens stay in the OS keychain."
      >
        <div className="space-y-3">
          {googleConnections.length > 0 ? (
            <ItemGroup className="gap-2">
              {googleConnections.map((connection) => (
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
                    {connection.status === "needs_reauth" ? (
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        disabled={isBusy}
                        onClick={() =>
                          void handleReconnect(
                            connection.accountId,
                            connection.accountLabel,
                          )
                        }
                      >
                        <RefreshCw className="size-4" />
                        Reconnect
                      </Button>
                    ) : null}
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
              No Google account connected yet.
            </p>
          )}

          <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end">
            <Field>
              <FieldLabel htmlFor="google-account-email">
                Google account email
              </FieldLabel>
              <Input
                id="google-account-email"
                type="email"
                value={accountEmail}
                placeholder="you@example.com"
                onChange={(event) => setAccountEmail(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Enter") {
                    void handleConnect();
                  }
                }}
              />
            </Field>
            <Button
              type="button"
              disabled={!accountEmail.trim() || isBusy}
              onClick={() => void handleConnect()}
            >
              {connectGoogle.isPending ? (
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
