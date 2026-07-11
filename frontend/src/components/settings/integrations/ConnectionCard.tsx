import { LogOut, RefreshCw } from "lucide-react";
import type { ReactNode } from "react";
import { Button } from "@/components/ui/button";
import {
  Item,
  ItemActions,
  ItemContent,
  ItemDescription,
  ItemTitle,
} from "@/components/ui/item";
import type { IntegrationConnection } from "@/lib/api";
import { ConnectionStatusBadge } from "./ConnectionStatusBadge";

export interface ConnectionCardProps {
  connection: IntegrationConnection;
  disabled?: boolean;
  onDisconnect: (accountId: string) => void;
  onReconnect?: (accountId: string, accountLabel: string) => void;
  secondaryAction?: {
    label: string;
    icon?: ReactNode;
    onClick: (accountId: string) => void;
  };
}

export function ConnectionCard({
  connection,
  disabled = false,
  onDisconnect,
  onReconnect,
  secondaryAction,
}: ConnectionCardProps) {
  return (
    <Item variant="outline">
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
        {connection.status === "needs_reauth" && onReconnect ? (
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={disabled}
            onClick={() =>
              void onReconnect(connection.accountId, connection.accountLabel)
            }
          >
            <RefreshCw className="size-4" />
            Reconnect
          </Button>
        ) : null}
        {secondaryAction ? (
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={disabled}
            onClick={() => void secondaryAction.onClick(connection.accountId)}
          >
            {secondaryAction.icon ?? <RefreshCw className="size-4" />}
            {secondaryAction.label}
          </Button>
        ) : null}
        <Button
          type="button"
          variant="ghost"
          size="sm"
          disabled={disabled}
          onClick={() => void onDisconnect(connection.accountId)}
        >
          <LogOut className="size-4" />
          Disconnect
        </Button>
      </ItemActions>
    </Item>
  );
}
