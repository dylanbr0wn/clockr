import {
  AlertCircle,
  CheckCircle2,
  LoaderCircle,
  LogOut,
  RefreshCw,
} from "lucide-react";
import { useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
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
            <div className="space-y-2">
              {googleConnections.map((connection) => (
                <div
                  key={connection.id}
                  className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-border px-3 py-2"
                >
                  <div className="min-w-0">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="truncate text-sm font-medium">
                        {connection.accountLabel}
                      </span>
                      <ConnectionStatusBadge status={connection.status} />
                    </div>
                    <p className="mt-0.5 truncate text-xs text-muted-foreground">
                      {connection.accountId}
                    </p>
                  </div>
                  <div className="flex gap-2">
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
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">
              No Google account connected yet.
            </p>
          )}

          <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end">
            <div className="grid gap-1.5">
              <Label htmlFor="google-account-email" className="text-xs">
                Google account email
              </Label>
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
            </div>
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

          {connectError ? (
            <p className="text-sm text-destructive">{connectError}</p>
          ) : null}
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
          <div className="space-y-2">
            {googleCalendars.map((calendar) => (
              <div
                key={calendar.id}
                className="grid gap-3 rounded-lg border border-border px-3 py-3 sm:grid-cols-[minmax(0,1fr)_auto_minmax(0,180px)] sm:items-center"
              >
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="truncate text-sm font-medium">
                      {calendar.name}
                    </span>
                    {calendar.isPrimary ? (
                      <span className="rounded-full bg-primary/10 px-2 py-0.5 text-[10px] font-medium text-primary">
                        Primary
                      </span>
                    ) : null}
                  </div>
                </div>

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
                  <SelectTrigger className="w-full bg-background" aria-label={`Default category for ${calendar.name}`}>
                    <SelectValue placeholder="Default category" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value={NONE_CATEGORY}>No default</SelectItem>
                    {categories.map((category) => (
                      <SelectItem key={category.id} value={String(category.id)}>
                        {category.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            ))}
          </div>
        )}
      </SettingBlock>
    </div>
  );
}
