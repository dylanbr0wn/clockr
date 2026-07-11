import { ArrowLeft } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  getIntegrationEntry,
  type IntegrationProviderId,
} from "./registry";

export function IntegrationDetail({
  providerId,
  onBack,
}: {
  providerId: IntegrationProviderId;
  onBack: () => void;
}) {
  const entry = getIntegrationEntry(providerId);
  if (!entry) {
    return (
      <div className="mx-auto max-w-2xl space-y-4 p-5">
        <Button type="button" variant="ghost" size="sm" onClick={onBack}>
          <ArrowLeft className="size-4" />
          Back to integrations
        </Button>
        <p className="text-sm text-muted-foreground">
          Unknown integration provider.
        </p>
      </div>
    );
  }

  const Panel = entry.Panel;

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="shrink-0 border-b border-border/70 bg-background px-5 py-3">
        <Button type="button" variant="ghost" size="sm" onClick={onBack}>
          <ArrowLeft className="size-4" />
          Back to integrations
        </Button>
        <h2 className="mt-2 text-lg font-semibold tracking-tight">
          {entry.displayName}
        </h2>
      </div>
      <div className="min-h-0 flex-1 overflow-auto px-5 py-5">
        <Panel />
      </div>
    </div>
  );
}
