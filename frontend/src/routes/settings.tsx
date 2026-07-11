import {
  createFileRoute,
  Link,
  Outlet,
} from "@tanstack/react-router";
import { Button } from "@/components/ui/button";
import {
  settingsNavItems,
  type SettingsSectionId,
} from "@/components/settings/settingsSections";
import { cn } from "@/lib/utils";
import { Sidebar, SidebarContent, SidebarGroup, SidebarGroupLabel, SidebarHeader, SidebarMenuButton, SidebarMenuItem, SidebarProvider } from "@/components/ui/sidebar";
import { ScrollArea } from "@/components/ui/scroll-area";
import { ArrowLeft, ChevronLeft } from "lucide-react";

export const Route = createFileRoute("/settings")({
  component: SettingsLayout,
});

const sectionPaths = {
  general: "/settings/general",
  integrations: "/settings/integrations",
  categories: "/settings/categories",
  ai: "/settings/ai",
  export: "/settings/export",
} as const satisfies Record<SettingsSectionId, string>;

function SettingsLayout() {
  return (
    <SidebarProvider>
      <SettingsSidebar />
      <ScrollArea className="h-[calc(100vh-48px)] w-full">
        <Outlet />
      </ScrollArea>
    </SidebarProvider>
  );
}

function SettingsSidebar() {
  return <Sidebar className="app-no-drag">
    <SidebarHeader className="mt-10">
      <SidebarMenuItem>
        <SidebarMenuButton asChild>
          <Link to="/">
            <ArrowLeft className="size-4 " />
            <span className="truncate">Back</span>
          </Link>
        </SidebarMenuButton>
      </SidebarMenuItem>
    </SidebarHeader>
    <SidebarContent>
      <SidebarGroup>
        <SidebarGroupLabel>Settings</SidebarGroupLabel>
        {settingsNavItems.map((section) => {
          const Icon = section.icon;

          if (!section.ready) {
            return (
              <SidebarMenuItem
                key={section.id}
              >
                <SidebarMenuButton disabled className="opacity-50 justify-start">
                  <Icon className="size-4 " />
                  <span className="truncate">{section.label}</span>
                </SidebarMenuButton>
              </SidebarMenuItem>
            );
          }

          return (
            <SidebarMenuItem key={section.id}>
              <SidebarMenuButton asChild className="justify-start hover:text-green-300">
                <Link
                  activeProps={{
                    className: cn("text-green-300 hover:text-green-300 bg-muted")
                  }}
                  to={sectionPaths[section.id]}
                >
                  <Icon className="size-4" />
                  <span className="truncate">{section.label}</span>
                </Link>
              </SidebarMenuButton>
            </SidebarMenuItem>
          );
        })}
      </SidebarGroup>
    </SidebarContent>
  </Sidebar>
}
