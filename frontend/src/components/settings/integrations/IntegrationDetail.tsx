import { Link } from "@tanstack/react-router";
import { ArrowLeft } from "lucide-react";
import { Button } from "@/components/ui/button";
import { getIntegrationEntry } from "./registry";

export function IntegrationDetail({ providerId }: { providerId: string }) {
  const entry = getIntegrationEntry(providerId);
  if (!entry) {
    return (
      <div className="mx-auto max-w-2xl space-y-4 p-5">
        <Button type="button" variant="ghost" size="sm" asChild>
          <Link to="/settings/integrations">
            <ArrowLeft className="size-4" />
            Back to integrations
          </Link>
        </Button>
        <p className="text-sm text-muted-foreground">
          Unknown integration provider.
        </p>
      </div>
    );
  }

  const Panel = entry.Panel;

  return (
    <div className="flex h-full min-h-0 flex-col w-full">
      <div className="min-h-0 flex-1 overflow-auto p-5">
        <Panel />
      </div>
    </div>
  );
}
