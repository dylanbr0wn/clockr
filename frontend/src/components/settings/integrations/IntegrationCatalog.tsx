import { ChevronRight } from "lucide-react";
import { useMemo } from "react";
import {
  Item,
  ItemActions,
  ItemContent,
  ItemGroup,
  ItemTitle,
} from "@/components/ui/item";
import { useIntegrationConnections } from "@/lib/api";
import type { IntegrationConnection } from "@/lib/api";
import { ConnectionStatusBadge } from "./ConnectionStatusBadge";
import {
  groupIntegrationsByKind,
  type IntegrationProviderId,
} from "./registry";

function aggregateProviderStatus(
  connections: IntegrationConnection[],
  providerId: IntegrationProviderId,
): string | null {
  const providerConnections = connections.filter(
    (connection) => connection.provider === providerId,
  );
  if (providerConnections.length === 0) {
    return null;
  }
  if (providerConnections.some((connection) => connection.status === "connected")) {
    return "connected";
  }
  if (
    providerConnections.some((connection) => connection.status === "needs_reauth")
  ) {
    return "needs_reauth";
  }
  if (
    providerConnections.some((connection) => connection.status === "disconnected")
  ) {
    return "disconnected";
  }
  return providerConnections[0]?.status ?? null;
}

export function IntegrationCatalog({
  onSelect,
}: {
  onSelect: (providerId: IntegrationProviderId) => void;
}) {
  const connectionsQuery = useIntegrationConnections();
  const connections = connectionsQuery.data ?? [];
  const groups = useMemo(() => groupIntegrationsByKind(), []);

  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <div>
        <h2 className="text-lg font-semibold tracking-tight">Integrations</h2>
        <p className="mt-1 text-sm text-muted-foreground">
          Connect calendar sources and activity evidence providers for schedule
          import and AI gap-fill.
        </p>
      </div>

      {groups.map((group) => (
        <section key={group.kind} className="space-y-2">
          <h3 className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
            {group.label}
          </h3>
          <ItemGroup className="gap-2">
            {group.entries.map((entry) => {
              const status = aggregateProviderStatus(connections, entry.id);
              return (
                <Item
                  key={entry.id}
                  variant="outline"
                  className="cursor-pointer hover:bg-muted/50"
                  onClick={() => onSelect(entry.id)}
                  onKeyDown={(event) => {
                    if (event.key === "Enter" || event.key === " ") {
                      event.preventDefault();
                      onSelect(entry.id);
                    }
                  }}
                  role="button"
                  tabIndex={0}
                >
                  <ItemContent className="min-w-0">
                    <ItemTitle className="flex flex-wrap items-center gap-2">
                      <span className="truncate">{entry.displayName}</span>
                      {status ? (
                        <ConnectionStatusBadge status={status} />
                      ) : (
                        <span className="rounded-full bg-muted px-2 py-0.5 text-[10px] font-medium text-muted-foreground">
                          Not connected
                        </span>
                      )}
                    </ItemTitle>
                  </ItemContent>
                  <ItemActions>
                    <ChevronRight className="size-4 text-muted-foreground" />
                  </ItemActions>
                </Item>
              );
            })}
          </ItemGroup>
        </section>
      ))}
    </div>
  );
}
