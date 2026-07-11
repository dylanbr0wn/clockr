import { createFileRoute } from "@tanstack/react-router";
import { IntegrationDetail } from "@/components/settings/integrations";

export const Route = createFileRoute("/settings/integrations/$providerId")({
  component: IntegrationDetailPage,
});

function IntegrationDetailPage() {
  const { providerId } = Route.useParams();
  return <IntegrationDetail providerId={providerId} />;
}
