import {
  LoaderCircle,
  Monitor,
  Moon,
  Sun,
} from "lucide-react";
import { useEffect, useState } from "react";
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
import { useLogPath, useRevealLogFolder } from "@/lib/api";
import { isShietAppAvailable } from "@/lib/api/shietService";
import { Environment } from "../../../wailsjs/runtime/runtime";
import { SettingBlock } from "./SettingBlock";
import { type ThemeSetting } from "./useConfiguredTheme";
import { useJsonSetting } from "./useJsonSetting";

type PeriodCadence = "weekly" | "bi-weekly" | "semi-monthly" | "monthly";
type EventHandling = "include" | "exclude" | "flag";

function EventHandlingSelect({
  label,
  value,
  onValueChange,
}: {
  label: string;
  value: EventHandling;
  onValueChange: (value: EventHandling) => void;
}) {
  return (
    <Field>
      <FieldLabel htmlFor={`setting-event-${label}`}>{label}</FieldLabel>
      <Select
        value={value}
        onValueChange={(nextValue) => onValueChange(nextValue as EventHandling)}
      >
        <SelectTrigger
          id={`setting-event-${label}`}
          className="w-full bg-background"
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="include">Include</SelectItem>
          <SelectItem value="flag">Flag</SelectItem>
          <SelectItem value="exclude">Exclude</SelectItem>
        </SelectContent>
      </Select>
    </Field>
  );
}

export function GeneralSettings() {
  const theme = useJsonSetting<ThemeSetting>("app.theme", "system");
  const cadence = useJsonSetting<PeriodCadence>(
    "period.cadence",
    "bi-weekly",
  );
  const acceptedEvents = useJsonSetting<EventHandling>(
    "events.accepted",
    "include",
  );
  const tentativeEvents = useJsonSetting<EventHandling>(
    "events.tentative",
    "flag",
  );
  const declinedEvents = useJsonSetting<EventHandling>(
    "events.declined",
    "exclude",
  );
  const logPath = useLogPath();
  const revealLogFolder = useRevealLogFolder();
  const [revealLabel, setRevealLabel] = useState("Open log folder");
  const [revealError, setRevealError] = useState<string | null>(null);
  const appAvailable = isShietAppAvailable();

  useEffect(() => {
    let mounted = true;
    void (async () => {
      try {
        const environment = await Environment();
        if (mounted && environment.platform === "darwin") {
          setRevealLabel("Reveal in Finder");
        }
      } catch {
        // Wails runtime absent in plain Vite.
      }
    })();
    return () => {
      mounted = false;
    };
  }, []);

  const isSaving =
    theme.isSaving ||
    cadence.isSaving ||
    acceptedEvents.isSaving ||
    tentativeEvents.isSaving ||
    declinedEvents.isSaving;
  const isLoading =
    theme.isLoading ||
    cadence.isLoading ||
    acceptedEvents.isLoading ||
    tentativeEvents.isLoading ||
    declinedEvents.isLoading;
  const error =
    theme.error ??
    cadence.error ??
    acceptedEvents.error ??
    tentativeEvents.error ??
    declinedEvents.error;

  const statusLabel = error
    ? "Unable to save settings"
    : isSaving
      ? "Saving"
      : isLoading
        ? "Loading"
        : "Saved";

  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <SettingBlock
        title="Period Defaults"
        description="New periods use this cadence when shiet opens the current range. Expected hours come from Work Schedule."
      >
        <Field className="max-w-xs">
          <FieldLabel htmlFor="setting-period-cadence">Cadence</FieldLabel>
          <Select
            value={cadence.value}
            onValueChange={(value) =>
              cadence.setValue(value as PeriodCadence)
            }
          >
            <SelectTrigger
              id="setting-period-cadence"
              className="w-full bg-background"
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="weekly">Weekly</SelectItem>
              <SelectItem value="bi-weekly">Bi-weekly</SelectItem>
              <SelectItem value="semi-monthly">Semi-monthly</SelectItem>
              <SelectItem value="monthly">Monthly</SelectItem>
            </SelectContent>
          </Select>
        </Field>
      </SettingBlock>

      <SettingBlock
        title="Appearance"
        description="shiet follows the system theme unless a theme is selected."
      >
        <Field className="max-w-xs">
          <FieldLabel htmlFor="setting-theme">Theme</FieldLabel>
          <Select
            value={theme.value}
            onValueChange={(value) => theme.setValue(value as ThemeSetting)}
          >
            <SelectTrigger
              id="setting-theme"
              className="w-full bg-background"
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="system">
                <span className="flex items-center gap-2">
                  <Monitor className="size-4" />
                  System
                </span>
              </SelectItem>
              <SelectItem value="light">
                <span className="flex items-center gap-2">
                  <Sun className="size-4" />
                  Light
                </span>
              </SelectItem>
              <SelectItem value="dark">
                <span className="flex items-center gap-2">
                  <Moon className="size-4" />
                  Dark
                </span>
              </SelectItem>
            </SelectContent>
          </Select>
        </Field>
      </SettingBlock>

      <SettingBlock
        title="Calendar Event Defaults"
        description="Imported event states determine how they contribute to schedule gaps."
      >
        <div className="grid gap-3 sm:grid-cols-3">
          <EventHandlingSelect
            label="Accepted"
            value={acceptedEvents.value}
            onValueChange={acceptedEvents.setValue}
          />
          <EventHandlingSelect
            label="Tentative"
            value={tentativeEvents.value}
            onValueChange={tentativeEvents.setValue}
          />
          <EventHandlingSelect
            label="Declined"
            value={declinedEvents.value}
            onValueChange={declinedEvents.setValue}
          />
        </div>
      </SettingBlock>

      <SettingBlock
        title="Logs"
        description="Desktop diagnostics write JSON to this file. Open the folder to inspect or share logs."
      >
        <div className="space-y-3">
          <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end">
            <Field>
              <FieldLabel htmlFor="setting-log-path">Log file</FieldLabel>
              <Input
                id="setting-log-path"
                readOnly
                value={
                  logPath.data ||
                  (appAvailable
                    ? "Loading…"
                    : "Unavailable outside the desktop app")
                }
                className="bg-background font-mono text-xs"
              />
            </Field>
            <Button
              type="button"
              size="sm"
              disabled={
                !appAvailable || revealLogFolder.isPending || !logPath.data
              }
              onClick={() => {
                setRevealError(null);
                revealLogFolder.mutate(undefined, {
                  onError: (err) => {
                    setRevealError(
                      err instanceof Error
                        ? err.message
                        : "Could not open log folder",
                    );
                  },
                });
              }}
            >
              {revealLogFolder.isPending ? (
                <LoaderCircle className="size-4 animate-spin" />
              ) : (
                revealLabel
              )}
            </Button>
          </div>
          {revealError ? <FieldError>{revealError}</FieldError> : null}
          {!appAvailable ? (
            <p className="text-xs text-muted-foreground">
              Reveal requires the desktop app.
            </p>
          ) : null}
        </div>
      </SettingBlock>

      <p className="text-xs text-muted-foreground">{statusLabel}</p>
    </div>
  );
}
