import { LoaderCircle } from "lucide-react";
import {
  Item,
  ItemContent,
  ItemDescription,
  ItemGroup,
  ItemMedia,
  ItemTitle,
} from "@/components/ui/item";
import { ScrollArea } from "@/components/ui/scroll-area";
import type { GapEvidenceItem } from "@/lib/api";
import { errorMessage } from "@/lib/schedule";
import { evidenceBadgeClass, formatEvidenceLabel } from "./gapEvidence";

interface GapEvidencePreviewProps {
  items: GapEvidenceItem[];
  isLoading: boolean;
  error: unknown;
}

export function GapEvidencePreview({
  items,
  isLoading,
  error,
}: GapEvidencePreviewProps) {
  if (error) {
    return (
      <Item
        variant="outline"
        size="sm"
        className="border-destructive/30 bg-destructive/10"
      >
        <ItemContent>
          <ItemDescription className="text-destructive">
            {errorMessage(error)}
          </ItemDescription>
        </ItemContent>
      </Item>
    );
  }

  if (isLoading) {
    return (
      <Item variant="muted" size="sm">
        <ItemMedia variant="icon">
          <LoaderCircle className="animate-spin" />
        </ItemMedia>
        <ItemContent>
          <ItemDescription>Loading evidence…</ItemDescription>
        </ItemContent>
      </Item>
    );
  }

  if (items.length === 0) {
    return (
      <Item variant="muted" size="sm">
        <ItemContent>
          <ItemDescription>
            No activity evidence in this interval.
          </ItemDescription>
        </ItemContent>
      </Item>
    );
  }

  return (
    <div className="grid gap-2">
      <p className="text-xs font-medium text-muted-foreground">
        Evidence sent to the model ({items.length})
      </p>
      <ScrollArea className="max-h-36 rounded-md border border-border">
        <ItemGroup className="gap-0 divide-y divide-border p-1">
          {items.map((item, index) => (
            <Item
              key={`${item.provider}-${item.kind}-${index}`}
              variant="default"
              size="sm"
            >
              <ItemContent>
                <ItemTitle className="flex flex-wrap items-center gap-2 text-sm">
                  <span
                    className={`rounded-full px-2 py-0.5 text-xs font-medium ${evidenceBadgeClass(item.provider)}`}
                  >
                    {formatEvidenceLabel(item.provider, item.kind)}
                  </span>
                  {item.summary}
                </ItemTitle>
                {item.source ? (
                  <ItemDescription>{item.source}</ItemDescription>
                ) : null}
              </ItemContent>
            </Item>
          ))}
        </ItemGroup>
      </ScrollArea>
    </div>
  );
}
