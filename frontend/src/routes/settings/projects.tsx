import { createFileRoute } from "@tanstack/react-router";
import { ProjectSettings } from "@/components/settings/ProjectSettings";

export const Route = createFileRoute("/settings/projects")({
  component: ProjectsSettingsPage,
});

function ProjectsSettingsPage() {
  return (
    <div className="h-full min-h-0 overflow-auto p-5">
      <ProjectSettings />
    </div>
  );
}
