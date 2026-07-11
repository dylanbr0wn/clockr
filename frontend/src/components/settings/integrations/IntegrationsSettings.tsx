import { useState } from "react";
import { IntegrationCatalog } from "./IntegrationCatalog";
import { IntegrationDetail } from "./IntegrationDetail";
import type { IntegrationProviderId } from "./registry";

export function IntegrationsSettings() {
  const [selectedProviderId, setSelectedProviderId] =
    useState<IntegrationProviderId | null>(null);

  if (selectedProviderId) {
    return (
      <div className="flex h-full min-h-0 flex-col">
        <IntegrationDetail
          providerId={selectedProviderId}
          onBack={() => setSelectedProviderId(null)}
        />
      </div>
    );
  }

  return (
    <div className="h-full min-h-0 overflow-auto p-5">
      <IntegrationCatalog onSelect={setSelectedProviderId} />
    </div>
  );
}
