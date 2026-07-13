import { AlertCircle, Archive, LoaderCircle, Pencil, Plus, Trash2 } from "lucide-react";
import { useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Item,
  ItemActions,
  ItemContent,
  ItemDescription,
  ItemGroup,
  ItemTitle,
} from "@/components/ui/item";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  useArchiveProject,
  useCreateProject,
  useDeleteProject,
  useProjects,
  useUpdateProject,
} from "@/lib/api";
import type { CreateProjectInput, Project, UpdateProjectInput } from "@/lib/api/types";
import { CATEGORY_PALETTE, DEFAULT_CATEGORY_COLOR } from "@/lib/category/colors";
import { cn } from "@/lib/utils";
import { SettingBlock } from "./SettingBlock";

interface ProjectDraft {
  name: string;
  key: string;
  color: string;
}

const emptyDraft = (): ProjectDraft => ({
  name: "",
  key: "",
  color: DEFAULT_CATEGORY_COLOR,
});

function draftFromProject(project: Project): ProjectDraft {
  return {
    name: project.name,
    key: project.key === project.name ? "" : project.key,
    color: project.color,
  };
}

function projectUpdateInput(
  project: Project,
  overrides: Partial<UpdateProjectInput> = {},
): UpdateProjectInput {
  return {
    id: project.id,
    name: project.name,
    key: project.key,
    color: project.color,
    ...overrides,
  };
}

function ColorPaletteSwatches({
  value,
  label,
  disabled,
  pending,
  onSelect,
}: {
  value: string;
  label: string;
  disabled?: boolean;
  pending?: boolean;
  onSelect: (color: string) => void;
}) {
  return (
    <div className="flex flex-wrap items-center gap-1.5">
      {CATEGORY_PALETTE.map((color) => {
        const selected = value.toUpperCase() === color.toUpperCase();

        return (
          <button
            key={color}
            type="button"
            aria-label={`Set ${label} to ${color}`}
            disabled={disabled || pending}
            onClick={() => {
              if (!selected) {
                onSelect(color);
              }
            }}
            className={cn(
              "size-6 rounded-full border-4 transition-transform hover:scale-105 disabled:opacity-50",
              selected
                ? "border-foreground ring-2 ring-foreground/20"
                : "border-transparent",
            )}
            style={{ backgroundColor: color }}
          />
        );
      })}
    </div>
  );
}

function ProjectFormFields({
  draft,
  onChange,
  idPrefix,
  showColor = false,
}: {
  draft: ProjectDraft;
  onChange: (next: ProjectDraft) => void;
  idPrefix: string;
  showColor?: boolean;
}) {
  return (
    <div className="grid gap-3">
      <div className="grid gap-1.5">
        <Label htmlFor={`${idPrefix}-name`} className="text-xs">
          Name
        </Label>
        <Input
          id={`${idPrefix}-name`}
          value={draft.name}
          onChange={(event) => onChange({ ...draft, name: event.target.value })}
          placeholder="Client Alpha"
        />
      </div>
      {showColor ? (
        <div className="grid gap-1.5">
          <Label className="text-xs">Color</Label>
          <ColorPaletteSwatches
            value={draft.color}
            label={draft.name.trim() || "project"}
            onSelect={(color) => onChange({ ...draft, color })}
          />
        </div>
      ) : null}
      <div className="grid gap-1.5">
        <Label htmlFor={`${idPrefix}-key`} className="text-xs">
          Key (optional)
        </Label>
        <Input
          id={`${idPrefix}-key`}
          value={draft.key}
          onChange={(event) => onChange({ ...draft, key: event.target.value })}
          placeholder="Same as name"
        />
      </div>
    </div>
  );
}

export function ProjectSettings() {
  const projectsQuery = useProjects(true);
  const createProject = useCreateProject();
  const updateProject = useUpdateProject();
  const deleteProject = useDeleteProject();
  const archiveProject = useArchiveProject();

  const [editorOpen, setEditorOpen] = useState(false);
  const [editingProject, setEditingProject] = useState<Project | null>(null);
  const [draft, setDraft] = useState<ProjectDraft>(emptyDraft);
  const [formError, setFormError] = useState<string | null>(null);

  const projects = useMemo(
    () =>
      [...(projectsQuery.data ?? [])].sort((a, b) => {
        if (a.archived !== b.archived) {
          return a.archived ? 1 : -1;
        }
        return a.name.localeCompare(b.name);
      }),
    [projectsQuery.data],
  );

  const pendingProjectId = updateProject.isPending
    ? updateProject.variables?.id
    : undefined;

  const isBusy =
    createProject.isPending ||
    updateProject.isPending ||
    deleteProject.isPending ||
    archiveProject.isPending;

  const openCreate = () => {
    setEditingProject(null);
    setDraft(emptyDraft());
    setFormError(null);
    setEditorOpen(true);
  };

  const openEdit = (project: Project) => {
    setEditingProject(project);
    setDraft(draftFromProject(project));
    setFormError(null);
    setEditorOpen(true);
  };

  const handleSave = async () => {
    const name = draft.name.trim();
    if (!name) {
      setFormError("Name is required.");
      return;
    }

    setFormError(null);
    try {
      if (editingProject) {
        const input: UpdateProjectInput = {
          id: editingProject.id,
          name,
          key: draft.key.trim(),
          color: draft.color,
        };
        await updateProject.mutateAsync(input);
      } else {
        const input: CreateProjectInput = {
          name,
          key: draft.key.trim(),
          color: draft.color,
        };
        await createProject.mutateAsync(input);
      }
      setEditorOpen(false);
    } catch (error) {
      setFormError(
        error instanceof Error ? error.message : "Unable to save project",
      );
    }
  };

  const handleDelete = async (project: Project) => {
    try {
      await deleteProject.mutateAsync(project.id);
    } catch (error) {
      setFormError(
        error instanceof Error ? error.message : "Unable to delete project",
      );
    }
  };

  const handleArchive = async (project: Project) => {
    try {
      await archiveProject.mutateAsync(project.id);
    } catch (error) {
      setFormError(
        error instanceof Error ? error.message : "Unable to archive project",
      );
    }
  };

  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <SettingBlock
        title="Projects"
        description="Optional allocation targets for time entries. Unused projects can be deleted; referenced ones archive so history stays intact."
      >
        <div className="flex justify-end">
          <Button type="button" size="sm" onClick={openCreate} disabled={isBusy}>
            <Plus className="size-4" />
            Add project
          </Button>
        </div>

        {projectsQuery.isLoading ? (
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <LoaderCircle className="size-4 animate-spin" />
            Loading projects
          </div>
        ) : projects.length === 0 ? (
          <p className="text-sm text-muted-foreground">No projects yet.</p>
        ) : (
          <ItemGroup className="gap-2">
            {projects.map((project) => (
              <Item key={project.id} variant="outline">
                <ItemContent className="min-w-0">
                  <ItemTitle className="flex flex-wrap items-center gap-2">
                    {project.name}
                    {project.archived ? (
                      <span className="rounded-full bg-muted px-2 py-0.5 text-[10px] font-medium text-muted-foreground">
                        Archived
                      </span>
                    ) : null}
                    {project.key !== project.name ? (
                      <span className="rounded-full bg-muted px-2 py-0.5 text-[10px] font-medium text-muted-foreground">
                        key: {project.key}
                      </span>
                    ) : null}
                  </ItemTitle>
                  <ItemDescription className="text-xs text-muted-foreground">
                    {project.inUse ? "In use on time entries" : "Not referenced"}
                  </ItemDescription>
                </ItemContent>
                <ItemActions className="flex-col items-end gap-2">
                  <ColorPaletteSwatches
                    value={project.color}
                    label={project.name}
                    disabled={isBusy}
                    pending={pendingProjectId === project.id}
                    onSelect={(color) => {
                      updateProject.mutate(projectUpdateInput(project, { color }));
                    }}
                  />
                  <div className="flex items-center gap-1">
                    <Button
                      type="button"
                      size="icon-sm"
                      variant="ghost"
                      onClick={() => openEdit(project)}
                      disabled={isBusy}
                      aria-label={`Edit ${project.name}`}
                    >
                      <Pencil className="size-4" />
                    </Button>
                    {!project.archived && project.inUse ? (
                      <Button
                        type="button"
                        size="icon-sm"
                        variant="ghost"
                        onClick={() => void handleArchive(project)}
                        disabled={isBusy}
                        aria-label={`Archive ${project.name}`}
                      >
                        <Archive className="size-4" />
                      </Button>
                    ) : null}
                    {!project.inUse ? (
                      <Button
                        type="button"
                        size="icon-sm"
                        variant="ghost"
                        onClick={() => void handleDelete(project)}
                        disabled={isBusy}
                        aria-label={`Delete ${project.name}`}
                      >
                        <Trash2 className="size-4" />
                      </Button>
                    ) : null}
                  </div>
                </ItemActions>
              </Item>
            ))}
          </ItemGroup>
        )}

        {formError && !editorOpen ? (
          <p className="flex items-center gap-2 text-xs text-destructive">
            <AlertCircle className="size-3.5" />
            {formError}
          </p>
        ) : null}
      </SettingBlock>

      <Dialog open={editorOpen} onOpenChange={setEditorOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>
              {editingProject ? "Edit project" : "Add project"}
            </DialogTitle>
            <DialogDescription>
              Keys appear in exports. Leave blank to match the display name.
            </DialogDescription>
          </DialogHeader>

          <ProjectFormFields
            draft={draft}
            onChange={setDraft}
            idPrefix={editingProject ? "edit" : "create"}
            showColor
          />

          {formError ? (
            <p className="flex items-center gap-2 text-xs text-destructive">
              <AlertCircle className="size-3.5" />
              {formError}
            </p>
          ) : null}

          <DialogFooter>
            <Button
              type="button"
              variant="secondary"
              onClick={() => setEditorOpen(false)}
              disabled={isBusy}
            >
              Cancel
            </Button>
            <Button type="button" onClick={() => void handleSave()} disabled={isBusy}>
              {isBusy ? (
                <>
                  <LoaderCircle className="size-4 animate-spin" />
                  Saving
                </>
              ) : (
                "Save"
              )}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
