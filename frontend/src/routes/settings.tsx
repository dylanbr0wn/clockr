import {
  createFileRoute,
  Link,
  Outlet,
  useMatchRoute,
} from "@tanstack/react-router";
import {
  settingsNavItems,
  type SettingsSectionId,
} from "@/components/settings/settingsSections";
import {
  aggregateProviderStatus,
  integrationRegistry,
} from "@/components/settings/integrations";
import { useIntegrationConnections } from "@/lib/api";
import { Badge } from "@/components/ui/badge";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import {
  Sidebar,
  SidebarContent,
  SidebarGroup,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuSub,
  SidebarMenuSubButton,
  SidebarMenuSubItem,
  SidebarProvider,
} from "@/components/ui/sidebar";
import { ScrollArea } from "@/components/ui/scroll-area";
import { AlertCircle, ArrowLeft, CheckCircle2, ChevronRight } from "lucide-react";

export const Route = createFileRoute("/settings")({
  component: SettingsLayout,
});

const sectionPaths = {
  general: "/settings/general",
  integrations: "/settings/integrations",
  categories: "/settings/categories",
  projects: "/settings/projects",
  ai: "/settings/ai",
  privacy: "/settings/privacy",
  export: "/settings/export",
} as const satisfies Record<SettingsSectionId, string>;

const activeLinkClassName = "text-green-300 hover:text-green-300 bg-muted";

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

function IntegrationStatusBadge({ status }: { status: string | null }) {
  if (status === "connected") {
    return (
      <Badge
        variant="secondary"
        aria-label="Connected"
        className="ml-auto bg-emerald-500/10 px-1 text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-300"
      >
        <CheckCircle2 />
      </Badge>
    );
  }

  if (status === "needs_reauth") {
    return (
      <Badge
        variant="secondary"
        aria-label="Needs re-auth"
        className="ml-auto bg-amber-500/10 px-1 text-amber-700 dark:bg-amber-500/10 dark:text-amber-300"
      >
        <AlertCircle />
      </Badge>
    );
  }

  return null;
}

function SettingsSidebar() {
  const matchRoute = useMatchRoute();
  const connectionsQuery = useIntegrationConnections();
  const connections = connectionsQuery.data ?? [];
  const integrationsActive = Boolean(
    matchRoute({ to: "/settings/integrations", fuzzy: true }),
  );

  return (
    <Sidebar className="app-no-drag">
      <SidebarHeader className="mt-10">
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton asChild>
              <Link to="/">
                <ArrowLeft />
                <span>Back</span>
              </Link>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>
      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupLabel>Settings</SidebarGroupLabel>
          <SidebarMenu>
            {settingsNavItems.map((section) => {
              const Icon = section.icon;

              if (!section.ready) {
                return (
                  <SidebarMenuItem key={section.id}>
                    <SidebarMenuButton disabled className="opacity-50">
                      <Icon />
                      <span>{section.label}</span>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                );
              }

              if (section.id === "integrations") {
                return (
                  <Collapsible
                    key={section.id}
                    asChild
                    defaultOpen
                    className="group/collapsible"
                  >
                    <SidebarMenuItem>
                      <CollapsibleTrigger asChild>
                        <SidebarMenuButton
                          className="hover:text-green-300"
                          isActive={integrationsActive}
                        >
                          <Icon />
                          <span>{section.label}</span>
                          <ChevronRight className="ml-auto transition-transform duration-200 group-data-[state=open]/collapsible:rotate-90" />
                        </SidebarMenuButton>
                      </CollapsibleTrigger>
                      <CollapsibleContent>
                        <SidebarMenuSub>
                          {integrationRegistry.map((entry) => {
                            const status = aggregateProviderStatus(
                              connections,
                              entry.id,
                            );
                            return (
                              <SidebarMenuSubItem key={entry.id}>
                                <SidebarMenuSubButton
                                  asChild
                                  isActive={Boolean(
                                    matchRoute({
                                      to: "/settings/integrations/$providerId",
                                      params: { providerId: entry.id },
                                    }),
                                  )}
                                >
                                  <Link
                                    to="/settings/integrations/$providerId"
                                    params={{ providerId: entry.id }}
                                    activeProps={{
                                      className: activeLinkClassName,
                                    }}
                                  >
                                    <span>{entry.displayName}</span>
                                    <IntegrationStatusBadge status={status} />
                                  </Link>
                                </SidebarMenuSubButton>
                              </SidebarMenuSubItem>
                            );
                          })}
                        </SidebarMenuSub>
                      </CollapsibleContent>
                    </SidebarMenuItem>
                  </Collapsible>
                );
              }

              return (
                <SidebarMenuItem key={section.id}>
                  <SidebarMenuButton
                    asChild
                    className="hover:text-green-300"
                  >
                    <Link
                      activeProps={{ className: activeLinkClassName }}
                      to={sectionPaths[section.id]}
                    >
                      <Icon />
                      <span>{section.label}</span>
                    </Link>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              );
            })}
          </SidebarMenu>
        </SidebarGroup>
      </SidebarContent>
    </Sidebar>
  );
}
