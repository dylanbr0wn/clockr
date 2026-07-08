import { AlertCircle, LoaderCircle, Pencil, Plus, Trash2 } from "lucide-react";
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
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  useCategories,
  useCreateCategory,
  useDeleteCategory,
  useUpdateCategory,
} from "@/lib/api";
import type { Category, CreateCategoryInput, UpdateCategoryInput } from "@/lib/api/types";
import { SettingBlock } from "./SettingBlock";
import { Checkbox } from "../ui/checkbox";
import { Field, FieldContent, FieldDescription, FieldLabel, FieldTitle } from "../ui/field";

interface CategoryDraft {
  name: string;
  description: string;
  key: string;
  isDefaultGap: boolean;
}

const emptyDraft = (): CategoryDraft => ({
  name: "",
  description: "",
  key: "",
  isDefaultGap: false,
});

function draftFromCategory(category: Category): CategoryDraft {
  return {
    name: category.name,
    description: category.description,
    key: category.key === category.name ? "" : category.key,
    isDefaultGap: category.isDefaultGap,
  };
}

function CategoryFormFields({
  draft,
  onChange,
  idPrefix,
}: {
  draft: CategoryDraft;
  onChange: (next: CategoryDraft) => void;
  idPrefix: string;
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
          onChange={(event) =>
            onChange({ ...draft, name: event.target.value })
          }
          placeholder="Meetings"
        />
      </div>
      <div className="grid gap-1.5">
        <Label htmlFor={`${idPrefix}-description`} className="text-xs">
          Description for AI
        </Label>
        <textarea
          id={`${idPrefix}-description`}
          value={draft.description}
          onChange={(event) =>
            onChange({ ...draft, description: event.target.value })
          }
          placeholder="What belongs in this bucket? Examples help the model disambiguate."
          rows={3}
          className="w-full min-w-0 rounded-lg border border-input bg-transparent px-2.5 py-2 text-sm transition-colors outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 dark:bg-input/30"
        />
      </div>
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
      <FieldLabel>
        <Field orientation="horizontal">
          <Checkbox checked={draft.isDefaultGap} onCheckedChange={(checked) =>
            onChange({ ...draft, isDefaultGap: !!checked })} />
          <FieldContent>
            <FieldTitle>Default gap category</FieldTitle>
            <FieldDescription>
              Used when AI or gap-fill needs a fallback bucket.
            </FieldDescription>
          </FieldContent>
        </Field>
      </FieldLabel>
    </div>
  );
}

export function CategorySettings() {
  const categoriesQuery = useCategories();
  const createCategory = useCreateCategory();
  const updateCategory = useUpdateCategory();
  const deleteCategory = useDeleteCategory();

  const [editorOpen, setEditorOpen] = useState(false);
  const [editingCategory, setEditingCategory] = useState<Category | null>(null);
  const [draft, setDraft] = useState<CategoryDraft>(emptyDraft);
  const [formError, setFormError] = useState<string | null>(null);

  const categories = useMemo(
    () => [...(categoriesQuery.data ?? [])].sort((a, b) => a.name.localeCompare(b.name)),
    [categoriesQuery.data],
  );

  const isBusy =
    createCategory.isPending ||
    updateCategory.isPending ||
    deleteCategory.isPending;

  const openCreate = () => {
    setEditingCategory(null);
    setDraft(emptyDraft());
    setFormError(null);
    setEditorOpen(true);
  };

  const openEdit = (category: Category) => {
    setEditingCategory(category);
    setDraft(draftFromCategory(category));
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
      if (editingCategory) {
        const input: UpdateCategoryInput = {
          id: editingCategory.id,
          name,
          description: draft.description.trim(),
          key: draft.key.trim(),
          isDefaultGap: draft.isDefaultGap,
        };
        await updateCategory.mutateAsync(input);
      } else {
        const input: CreateCategoryInput = {
          name,
          description: draft.description.trim(),
          key: draft.key.trim(),
          isDefaultGap: draft.isDefaultGap,
        };
        await createCategory.mutateAsync(input);
      }
      setEditorOpen(false);
    } catch (error) {
      setFormError(
        error instanceof Error ? error.message : "Unable to save category",
      );
    }
  };

  const handleDelete = async (category: Category) => {
    if (category.isDefaultGap) {
      setFormError("Set another default-gap category before deleting this one.");
      return;
    }

    try {
      await deleteCategory.mutateAsync(category.id);
    } catch (error) {
      setFormError(
        error instanceof Error ? error.message : "Unable to delete category",
      );
    }
  };

  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <SettingBlock
        title="Categories"
        description="Name, describe, and key your time buckets. AI sync and gap-fill receive these definitions — including descriptions — when suggesting categories."
      >
        <div className="rounded-lg border border-amber-500/30 bg-amber-500/5 px-3 py-2 text-xs text-amber-900 dark:text-amber-200">
          Cloud AI models receive category names and descriptions. Avoid sensitive
          client details unless you use a local model.
        </div>

        <div className="flex justify-end">
          <Button type="button" size="sm" onClick={openCreate} disabled={isBusy}>
            <Plus className="size-4" />
            Add category
          </Button>
        </div>

        {categoriesQuery.isLoading ? (
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <LoaderCircle className="size-4 animate-spin" />
            Loading categories
          </div>
        ) : categories.length === 0 ? (
          <p className="text-sm text-muted-foreground">No categories yet.</p>
        ) : (
          <div className="divide-y divide-border rounded-lg border border-border">
            {categories.map((category) => (
              <div
                key={category.id}
                className="flex items-start justify-between gap-3 px-3 py-3"
              >
                <div className="min-w-0 space-y-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <p className="text-sm font-medium">{category.name}</p>
                    {category.isDefaultGap ? (
                      <span className="rounded-full bg-primary/10 px-2 py-0.5 text-[10px] font-medium text-primary">
                        Default gap
                      </span>
                    ) : null}
                    {category.key !== category.name ? (
                      <span className="rounded-full bg-muted px-2 py-0.5 text-[10px] font-medium text-muted-foreground">
                        key: {category.key}
                      </span>
                    ) : null}
                  </div>
                  {category.description ? (
                    <p className="line-clamp-2 text-xs text-muted-foreground">
                      {category.description}
                    </p>
                  ) : (
                    <p className="text-xs text-muted-foreground italic">
                      No AI description
                    </p>
                  )}
                </div>
                <div className="flex shrink-0 items-center gap-1">
                  <Button
                    type="button"
                    size="icon-sm"
                    variant="ghost"
                    onClick={() => openEdit(category)}
                    disabled={isBusy}
                    aria-label={`Edit ${category.name}`}
                  >
                    <Pencil className="size-4" />
                  </Button>
                  <Button
                    type="button"
                    size="icon-sm"
                    variant="ghost"
                    onClick={() => void handleDelete(category)}
                    disabled={isBusy || category.isDefaultGap}
                    aria-label={`Delete ${category.name}`}
                  >
                    <Trash2 className="size-4" />
                  </Button>
                </div>
              </div>
            ))}
          </div>
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
              {editingCategory ? "Edit category" : "Add category"}
            </DialogTitle>
            <DialogDescription>
              Keys are what the AI returns. Leave key blank to match the display
              name.
            </DialogDescription>
          </DialogHeader>

          <CategoryFormFields
            draft={draft}
            onChange={setDraft}
            idPrefix={editingCategory ? "edit" : "create"}
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
