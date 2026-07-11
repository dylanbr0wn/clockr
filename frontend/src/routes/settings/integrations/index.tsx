import { createFileRoute } from "@tanstack/react-router";
import { IntegrationCatalog } from "@/components/settings/integrations";

export const Route = createFileRoute("/settings/integrations/")({
  component: IntegrationsCatalogPage,
});

function IntegrationsCatalogPage() {
  return (
    <div className="h-full min-h-0 overflow-auto p-5">
      <IntegrationCatalog />
    </div>
  );
}
