import {
  CalendarClock,
  Download,
  Plug,
  Settings,
  Shield,
  Sparkles,
  Tags,
  type LucideIcon,
} from "lucide-react";

export type SettingsSectionId =
  | "general"
  | "workSchedule"
  | "integrations"
  | "categories"
  | "ai"
  | "privacy"
  | "export";

export const settingsNavItems: Array<{
  id: SettingsSectionId;
  label: string;
  icon: LucideIcon;
  ready: true;
}> = [
  { id: "general", label: "General", icon: Settings, ready: true },
  { id: "workSchedule", label: "Work Schedule", icon: CalendarClock, ready: true },
  { id: "integrations", label: "Integrations", icon: Plug, ready: true },
  { id: "categories", label: "Categories", icon: Tags, ready: true },
  { id: "ai", label: "AI Model", icon: Sparkles, ready: true },
  { id: "privacy", label: "Privacy", icon: Shield, ready: true },
  { id: "export", label: "Export", icon: Download, ready: true },
];
