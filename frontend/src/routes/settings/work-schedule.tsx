import { createFileRoute } from "@tanstack/react-router";
import { WorkScheduleSettings } from "@/components/settings/WorkScheduleSettings";

export const Route = createFileRoute("/settings/work-schedule")({
  component: WorkScheduleSettingsPage,
});

function WorkScheduleSettingsPage() {
  return (
    <div className="h-full min-h-0 overflow-auto p-5">
      <WorkScheduleSettings />
    </div>
  );
}
