import {
  CalendarDays,
  Download,
  Monitor,
  Moon,
  Settings,
  Shield,
  Sparkles,
  Sun,
  Tags,
} from "lucide-react";
import {
  useEffect,
  useMemo,
  useState,
  type ComponentType,
  type ReactNode,
} from "react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogFooter,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { useSetSetting, useSetting } from "@/lib/api";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "../ui/tabs";
import { AIModelSettings } from "./AIModelSettings";
import { CalendarSettings } from "./CalendarSettings";
import { ExportSettings } from "./ExportSettings";
import { SettingBlock } from "./SettingBlock";

type ThemeSetting = "system" | "light" | "dark";
type PeriodCadence = "weekly" | "bi-weekly" | "semi-monthly" | "monthly";
type EventHandling = "include" | "exclude" | "flag";

interface SettingsDialogProps {
  children: ReactNode;
}

const sections: Array<{
  id: string;
  label: string;
  icon: ComponentType<{ className?: string }>;
  ready: boolean;
}> = [
  { id: "general", label: "General", icon: Settings, ready: true },
  { id: "calendars", label: "Calendars", icon: CalendarDays, ready: true },
  { id: "categories", label: "Categories", icon: Tags, ready: false },
  { id: "ai", label: "AI Model", icon: Sparkles, ready: true },
  { id: "privacy", label: "Privacy", icon: Shield, ready: false },
  { id: "export", label: "Export", icon: Download, ready: true },
];

function parseJsonSetting<T>(raw: string | null | undefined, fallback: T): T {
  if (!raw) {
    return fallback;
  }

  try {
    return JSON.parse(raw) as T;
  } catch {
    return fallback;
  }
}

function useJsonSetting<T>(key: string, fallback: T) {
  const query = useSetting(key);
  const mutation = useSetSetting();
  const value = useMemo(
    () => parseJsonSetting(query.data, fallback),
    [fallback, query.data],
  );

  return {
    error: query.error ?? mutation.error,
    isLoading: query.isLoading,
    isSaving: mutation.isPending,
    setValue: (nextValue: T) => {
      mutation.mutate({ key, value: JSON.stringify(nextValue) });
    },
    value,
  };
}

function useConfiguredTheme(theme: ThemeSetting) {
  useEffect(() => {
    const media = window.matchMedia("(prefers-color-scheme: dark)");

    const applyTheme = () => {
      const resolvedTheme =
        theme === "system" ? (media.matches ? "dark" : "light") : theme;
      document.documentElement.classList.toggle(
        "dark",
        resolvedTheme === "dark",
      );
    };

    applyTheme();
    media.addEventListener("change", applyTheme);

    return () => {
      media.removeEventListener("change", applyTheme);
    };
  }, [theme]);
}

function EditableNumberSetting({
  id,
  label,
  min,
  max,
  step,
  value,
  onCommit,
}: {
  id: string;
  label: string;
  min: number;
  max: number;
  step: number;
  value: number;
  onCommit: (value: number) => void;
}) {
  const [draft, setDraft] = useState(String(value));

  useEffect(() => {
    setDraft(String(value));
  }, [value]);

  const commitDraft = () => {
    const parsed = Number(draft);
    if (!Number.isFinite(parsed)) {
      setDraft(String(value));
      return;
    }

    const clamped = Math.min(Math.max(parsed, min), max);
    setDraft(String(clamped));
    onCommit(clamped);
  };

  return (
    <div className="grid gap-1.5">
      <Label htmlFor={id} className="text-xs">
        {label}
      </Label>
      <Input
        id={id}
        type="number"
        min={min}
        max={max}
        step={step}
        value={draft}
        onBlur={commitDraft}
        onChange={(event) => setDraft(event.target.value)}
      />
    </div>
  );
}

function EditableTimeSetting({
  id,
  label,
  value,
  onCommit,
}: {
  id: string;
  label: string;
  value: string;
  onCommit: (value: string) => void;
}) {
  const [draft, setDraft] = useState(value);

  useEffect(() => {
    setDraft(value);
  }, [value]);

  return (
    <div className="grid gap-1.5">
      <Label htmlFor={id} className="text-xs">
        {label}
      </Label>
      <Input
        id={id}
        type="time"
        value={draft}
        onBlur={() => onCommit(draft)}
        onChange={(event) => setDraft(event.target.value)}
      />
    </div>
  );
}

export function SettingsDialog({ children }: SettingsDialogProps) {
  const theme = useJsonSetting<ThemeSetting>("app.theme", "system");
  const cadence = useJsonSetting<PeriodCadence>(
    "period.cadence",
    "bi-weekly",
  );
  const targetHours = useJsonSetting<number>("period.target_hours", 8);
  const windowStart = useJsonSetting<string>("window.start", "09:00");
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

  useConfiguredTheme(theme.value);

  const isSaving =
    theme.isSaving ||
    cadence.isSaving ||
    targetHours.isSaving ||
    windowStart.isSaving ||
    acceptedEvents.isSaving ||
    tentativeEvents.isSaving ||
    declinedEvents.isSaving;
  const isLoading =
    theme.isLoading ||
    cadence.isLoading ||
    targetHours.isLoading ||
    windowStart.isLoading ||
    acceptedEvents.isLoading ||
    tentativeEvents.isLoading ||
    declinedEvents.isLoading;
  const error =
    theme.error ??
    cadence.error ??
    targetHours.error ??
    windowStart.error ??
    acceptedEvents.error ??
    tentativeEvents.error ??
    declinedEvents.error;

  return (
    <Dialog>
      <DialogTrigger asChild>{children}</DialogTrigger>
      <DialogContent className="app-no-drag h-[min(720px,calc(100vh-2rem))] max-w-4xl grid-rows-[auto_minmax(0,1fr)_auto] gap-0 overflow-hidden p-0!">
        {/*<DialogHeader className="border-b border-border px-5 py-4 pr-12">
          <DialogTitle>Settings</DialogTitle>
          <DialogDescription>
            General app defaults and period behavior.
          </DialogDescription>
        </DialogHeader>*/}

        <Tabs className="grid min-h-0 grid-cols-[180px_minmax(0,1fr)] h-[min(720px,calc(100vh-2rem))]" orientation="vertical" defaultValue="general">
          <div className="border-r border-border p-1 bg-muted h-full">
            <TabsList className=" rounded-none h-full w-full bg-muted">
              {sections.map((section) => {
                const Icon = section.icon;

                return (
                  <TabsTrigger
                    value={section.id}
                    key={section.id}
                    type="button"
                    disabled={!section.ready}
                  >
                    <Icon className="size-4" />
                    <span className="truncate">{section.label}</span>
                  </TabsTrigger>
                );
              })}
            </TabsList>
          </div>
          <div className="h-[min(720px,calc(100vh-2rem))] grid grid-rows-[1fr_auto]">
            <TabsContent value="general" className="min-h-0 overflow-auto p-5">
              <div className="mx-auto max-w-2xl space-y-6">
                <SettingBlock
                  title="Period Defaults"
                  description="New periods use these values when Clockr opens the current range."
                >
                  <div className="grid gap-3 sm:grid-cols-3">
                    <div className="grid gap-1.5">
                      <Label htmlFor="setting-period-cadence" className="text-xs">
                        Cadence
                      </Label>
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
                          <SelectItem value="semi-monthly">
                            Semi-monthly
                          </SelectItem>
                          <SelectItem value="monthly">Monthly</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                    <EditableNumberSetting
                      id="setting-target-hours"
                      label="Target hours"
                      min={1}
                      max={24}
                      step={0.25}
                      value={targetHours.value}
                      onCommit={targetHours.setValue}
                    />
                    <EditableTimeSetting
                      id="setting-window-start"
                      label="Workday starts"
                      value={windowStart.value}
                      onCommit={windowStart.setValue}
                    />
                  </div>
                </SettingBlock>

                <SettingBlock
                  title="Appearance"
                  description="Clockr follows the system theme unless a theme is selected."
                >
                  <div className="grid max-w-xs gap-1.5">
                    <Label htmlFor="setting-theme" className="text-xs">
                      Theme
                    </Label>
                    <Select
                      value={theme.value}
                      onValueChange={(value) =>
                        theme.setValue(value as ThemeSetting)
                      }
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
                  </div>
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
              </div>
            </TabsContent>
            <TabsContent value="calendars" className="min-h-0 overflow-auto p-5">
              <CalendarSettings />
            </TabsContent>
            <TabsContent value="ai" className="min-h-0 overflow-auto p-5">
              <AIModelSettings />
            </TabsContent>
            <TabsContent value="export" className="min-h-0 overflow-auto p-5">
              <ExportSettings />
            </TabsContent>
            <DialogFooter className="border-t border-border px-5 py-3">
              <div className="flex min-h-8 w-full items-center justify-between gap-3">
                <p className="truncate text-xs text-muted-foreground">
                  {error
                    ? "Unable to save settings"
                    : isSaving
                      ? "Saving"
                      : isLoading
                        ? "Loading"
                        : "Saved"}
                </p>
                <Separator orientation="vertical" className="h-5" />
                <DialogClose asChild>
                  <Button type="button" variant="secondary" size="sm">
                    Done
                  </Button>
                </DialogClose>
              </div>
            </DialogFooter>
          </div>

        </Tabs>


      </DialogContent>
    </Dialog>
  );
}

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
    <div className="grid gap-1.5">
      <Label htmlFor={`setting-event-${label}`} className="text-xs">
        {label}
      </Label>
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
    </div>
  );
}
