import {
  AlertCircle,
  ChevronDown,
  ChevronUp,
  Copy,
  LoaderCircle,
  Pencil,
  Plus,
  Trash2,
} from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import {
  Item,
  ItemActions,
  ItemContent,
  ItemDescription,
  ItemGroup,
  ItemTitle,
} from "@/components/ui/item";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  useCreateExportTemplate,
  useCurrentPeriod,
  useDeleteExportTemplate,
  useDuplicateExportTemplate,
  useExportTemplates,
  usePreviewExport,
  useUpdateExportTemplate,
  type ExportTemplate,
} from "@/lib/api";
import { isShietAppAvailable } from "@/lib/api/shietService";
import {
  defaultTabularSpec,
  DEFAULT_TEXT_TEMPLATE_BODY,
  encodeTabularSpec,
  fieldCatalog,
  formatFromSpec,
  isTabularFormat,
  isTextFormat,
  parseTabularSpec,
  usePeriodExport,
  type ExportGrain,
  type ExportLayout,
  type TabularColumnSpec,
  type TabularTemplateSpec,
} from "@/lib/export";
import { localDateKey } from "@/lib/schedule";
import { defaultTimeZone } from "@/lib/schedule/timezone";
import { ExportActions } from "@/components/export/ExportActions";
import { PeriodStatsPanel } from "@/components/stats/PeriodStatsPanel";
import { SettingBlock } from "./SettingBlock";

interface TemplateDraft {
  name: string;
  description: string;
  spec: TabularTemplateSpec;
}

interface TextTemplateDraft {
  name: string;
  description: string;
  body: string;
}

function emptyDraft(): TemplateDraft {
  return {
    name: "",
    description: "",
    spec: defaultTabularSpec("rollup", "flat"),
  };
}

function emptyTextDraft(): TextTemplateDraft {
  return {
    name: "",
    description: "",
    body: DEFAULT_TEXT_TEMPLATE_BODY,
  };
}

function draftFromTemplate(template: ExportTemplate): TemplateDraft {
  const parsed = parseTabularSpec(template.body);
  const spec =
    parsed ??
    defaultTabularSpec(
      template.key.includes("detail") ? "detail" : "rollup",
      template.key.includes("matrix") ? "matrix" : "flat",
    );
  if (template.format === "tsv") {
    spec.delimiter = "\t";
  } else if (template.format === "csv") {
    spec.delimiter = ",";
  }
  return {
    name: template.name,
    description: template.description,
    spec,
  };
}

function textDraftFromTemplate(template: ExportTemplate): TextTemplateDraft {
  return {
    name: template.name,
    description: template.description,
    body: template.body.trim() ? template.body : DEFAULT_TEXT_TEMPLATE_BODY,
  };
}

function pruneColumnsToCatalog(
  columns: TabularColumnSpec[],
  grain: ExportGrain,
  layout: ExportLayout,
): TabularColumnSpec[] {
  const allowed = new Set(fieldCatalog(grain, layout).map((field) => field.field));
  const kept = columns.filter((column) => allowed.has(column.field));
  if (kept.length > 0) {
    return kept;
  }
  return defaultTabularSpec(grain, layout).columns;
}

export function ExportSettings() {
  const today = useMemo(() => localDateKey(), []);
  const timeZone = useMemo(() => defaultTimeZone(), []);
  const currentPeriodQuery = useCurrentPeriod(today, timeZone);
  const period = currentPeriodQuery.data ?? null;
  const exportData = usePeriodExport(period);

  const templatesQuery = useExportTemplates();
  const createTemplate = useCreateExportTemplate();
  const updateTemplate = useUpdateExportTemplate();
  const deleteTemplate = useDeleteExportTemplate();
  const duplicateTemplate = useDuplicateExportTemplate();

  const [editorOpen, setEditorOpen] = useState(false);
  const [editingTemplate, setEditingTemplate] = useState<ExportTemplate | null>(
    null,
  );
  const [draft, setDraft] = useState<TemplateDraft>(emptyDraft);
  const [formError, setFormError] = useState<string | null>(null);
  const [debouncedBody, setDebouncedBody] = useState("");

  const [textEditorOpen, setTextEditorOpen] = useState(false);
  const [editingTextTemplate, setEditingTextTemplate] =
    useState<ExportTemplate | null>(null);
  const [textDraft, setTextDraft] = useState<TextTemplateDraft>(emptyTextDraft);
  const [textFormError, setTextFormError] = useState<string | null>(null);
  const [debouncedTextBody, setDebouncedTextBody] = useState("");

  const tabularTemplates = useMemo(
    () =>
      (templatesQuery.data ?? []).filter((template) =>
        isTabularFormat(template.format),
      ),
    [templatesQuery.data],
  );

  const textTemplates = useMemo(
    () =>
      (templatesQuery.data ?? []).filter((template) =>
        isTextFormat(template.format),
      ),
    [templatesQuery.data],
  );

  const catalog = useMemo(
    () => fieldCatalog(draft.spec.grain, draft.spec.layout),
    [draft.spec.grain, draft.spec.layout],
  );

  const encodedBody = useMemo(
    () => encodeTabularSpec(draft.spec),
    [draft.spec],
  );

  useEffect(() => {
    const handle = window.setTimeout(() => {
      setDebouncedBody(encodedBody);
    }, 250);
    return () => window.clearTimeout(handle);
  }, [encodedBody]);

  useEffect(() => {
    const handle = window.setTimeout(() => {
      setDebouncedTextBody(textDraft.body);
    }, 250);
    return () => window.clearTimeout(handle);
  }, [textDraft.body]);

  const previewQuery = usePreviewExport(
    {
      periodId: period?.id,
      format: formatFromSpec(draft.spec),
      body: debouncedBody,
    },
    editorOpen && isShietAppAvailable() && Boolean(period?.id),
  );

  const textPreviewQuery = usePreviewExport(
    {
      periodId: period?.id,
      format: "text",
      body: debouncedTextBody,
    },
    textEditorOpen &&
      isShietAppAvailable() &&
      Boolean(period?.id) &&
      Boolean(debouncedTextBody.trim()),
  );

  const isBusy =
    createTemplate.isPending ||
    updateTemplate.isPending ||
    deleteTemplate.isPending ||
    duplicateTemplate.isPending;

  const openCreate = () => {
    setEditingTemplate(null);
    setDraft(emptyDraft());
    setFormError(null);
    setEditorOpen(true);
  };

  const openEdit = (template: ExportTemplate) => {
    if (template.builtin) {
      return;
    }
    setEditingTemplate(template);
    setDraft(draftFromTemplate(template));
    setFormError(null);
    setEditorOpen(true);
  };

  const openCreateText = () => {
    setEditingTextTemplate(null);
    setTextDraft(emptyTextDraft());
    setTextFormError(null);
    setTextEditorOpen(true);
  };

  const openEditText = (template: ExportTemplate) => {
    if (template.builtin) {
      return;
    }
    setEditingTextTemplate(template);
    setTextDraft(textDraftFromTemplate(template));
    setTextFormError(null);
    setTextEditorOpen(true);
  };

  const setGrain = (grain: ExportGrain) => {
    setDraft((current) => {
      const layout = grain === "detail" ? "flat" : current.spec.layout;
      return {
        ...current,
        spec: {
          ...current.spec,
          grain,
          layout,
          columns: pruneColumnsToCatalog(current.spec.columns, grain, layout),
        },
      };
    });
  };

  const setLayout = (layout: ExportLayout) => {
    setDraft((current) => {
      if (current.spec.grain === "detail") {
        return current;
      }
      return {
        ...current,
        spec: {
          ...current.spec,
          layout,
          columns: pruneColumnsToCatalog(
            current.spec.columns,
            current.spec.grain,
            layout,
          ),
        },
      };
    });
  };

  const toggleField = (field: string, checked: boolean) => {
    setDraft((current) => {
      if (checked) {
        if (current.spec.columns.some((column) => column.field === field)) {
          return current;
        }
        return {
          ...current,
          spec: {
            ...current.spec,
            columns: [
              ...current.spec.columns,
              {
                field,
                header:
                  catalog.find((item) => item.field === field)?.label ?? field,
              },
            ],
          },
        };
      }
      return {
        ...current,
        spec: {
          ...current.spec,
          columns: current.spec.columns.filter(
            (column) => column.field !== field,
          ),
        },
      };
    });
  };

  const moveColumn = (index: number, delta: number) => {
    setDraft((current) => {
      const nextIndex = index + delta;
      if (nextIndex < 0 || nextIndex >= current.spec.columns.length) {
        return current;
      }
      const columns = [...current.spec.columns];
      const [item] = columns.splice(index, 1);
      columns.splice(nextIndex, 0, item);
      return { ...current, spec: { ...current.spec, columns } };
    });
  };

  const updateColumnHeader = (index: number, header: string) => {
    setDraft((current) => {
      const columns = current.spec.columns.map((column, columnIndex) =>
        columnIndex === index ? { ...column, header } : column,
      );
      return { ...current, spec: { ...current.spec, columns } };
    });
  };

  const handleSave = async () => {
    const name = draft.name.trim();
    if (!name) {
      setFormError("Name is required.");
      return;
    }
    if (draft.spec.columns.length === 0) {
      setFormError("Pick at least one column.");
      return;
    }
    if (
      draft.spec.layout === "matrix" &&
      !draft.spec.columns.some(
        (column) =>
          column.field === "category_name" || column.field === "category_key",
      )
    ) {
      setFormError("Matrix layout needs category name or key.");
      return;
    }

    setFormError(null);
    const format = formatFromSpec(draft.spec);
    const body = encodeTabularSpec(draft.spec);

    try {
      if (editingTemplate) {
        await updateTemplate.mutateAsync({
          id: editingTemplate.id,
          name,
          description: draft.description.trim(),
          format,
          body,
        });
      } else {
        await createTemplate.mutateAsync({
          name,
          description: draft.description.trim(),
          format,
          body,
        });
      }
      setEditorOpen(false);
      toast.success(editingTemplate ? "Template updated" : "Template created");
    } catch (error) {
      setFormError(
        error instanceof Error ? error.message : "Unable to save template",
      );
    }
  };

  const handleSaveText = async () => {
    const name = textDraft.name.trim();
    if (!name) {
      setTextFormError("Name is required.");
      return;
    }
    if (!textDraft.body.trim()) {
      setTextFormError("Template body is required.");
      return;
    }

    setTextFormError(null);
    try {
      if (editingTextTemplate) {
        await updateTemplate.mutateAsync({
          id: editingTextTemplate.id,
          name,
          description: textDraft.description.trim(),
          format: "text",
          body: textDraft.body,
        });
      } else {
        await createTemplate.mutateAsync({
          name,
          description: textDraft.description.trim(),
          format: "text",
          body: textDraft.body,
        });
      }
      setTextEditorOpen(false);
      toast.success(
        editingTextTemplate ? "Text template updated" : "Text template created",
      );
    } catch (error) {
      setTextFormError(
        error instanceof Error ? error.message : "Unable to save template",
      );
    }
  };

  const handleDuplicate = async (template: ExportTemplate) => {
    try {
      const copy = await duplicateTemplate.mutateAsync(template.key);
      toast.success("Template duplicated", { description: copy.name });
    } catch (error) {
      toast.error("Duplicate failed", {
        description:
          error instanceof Error ? error.message : "Unable to duplicate",
      });
    }
  };

  const handleDelete = async (template: ExportTemplate) => {
    if (template.builtin) {
      return;
    }
    try {
      await deleteTemplate.mutateAsync(template.id);
      toast.success("Template deleted");
    } catch (error) {
      toast.error("Delete failed", {
        description:
          error instanceof Error ? error.message : "Unable to delete",
      });
    }
  };

  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <SettingBlock
        title="Period export"
        description="Review period stats, then copy a text summary or download a CSV for the current pay period."
      >
        <div className="flex flex-col gap-4">
          {exportData.isLoading || currentPeriodQuery.isLoading ? (
            <p className="text-sm text-muted-foreground">Loading period totals</p>
          ) : (
            <PeriodStatsPanel summary={exportData.summary} today={today} />
          )}
          <ExportActions
            summary={exportData.summary}
            items={exportData.items}
            periodId={period?.id}
            disabled={exportData.isLoading || currentPeriodQuery.isLoading}
            layout="stacked"
          />
        </div>
      </SettingBlock>

      <SettingBlock
        title="CSV / TSV templates"
        description="Create custom tabular layouts from the field catalog. Builtin presets stay read-only — duplicate to customize."
      >
        <div className="flex justify-end">
          <Button
            type="button"
            size="sm"
            onClick={openCreate}
            disabled={isBusy || !isShietAppAvailable()}
          >
            <Plus className="size-4" />
            New template
          </Button>
        </div>

        {!isShietAppAvailable() ? (
          <p className="text-sm text-muted-foreground">
            Template editing requires the desktop app.
          </p>
        ) : templatesQuery.isLoading ? (
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <LoaderCircle className="size-4 animate-spin" />
            Loading templates
          </div>
        ) : tabularTemplates.length === 0 ? (
          <p className="text-sm text-muted-foreground">No tabular templates yet.</p>
        ) : (
          <ItemGroup className="gap-2">
            {tabularTemplates.map((template) => (
              <Item key={template.id} variant="outline">
                <ItemContent className="min-w-0">
                  <ItemTitle className="flex flex-wrap items-center gap-2">
                    {template.name}
                    {template.builtin ? (
                      <span className="rounded-full bg-muted px-2 py-0.5 text-[10px] font-medium text-muted-foreground">
                        Builtin
                      </span>
                    ) : null}
                    <span className="rounded-full bg-muted px-2 py-0.5 text-[10px] font-medium uppercase text-muted-foreground">
                      {template.format}
                    </span>
                  </ItemTitle>
                  <ItemDescription className="line-clamp-2 text-xs">
                    {template.description || template.key}
                  </ItemDescription>
                </ItemContent>
                <ItemActions className="flex items-center gap-1">
                  <Button
                    type="button"
                    size="icon-sm"
                    variant="ghost"
                    onClick={() => void handleDuplicate(template)}
                    disabled={isBusy}
                    aria-label={`Duplicate ${template.name}`}
                  >
                    <Copy className="size-4" />
                  </Button>
                  <Button
                    type="button"
                    size="icon-sm"
                    variant="ghost"
                    onClick={() => openEdit(template)}
                    disabled={isBusy || template.builtin}
                    aria-label={`Edit ${template.name}`}
                  >
                    <Pencil className="size-4" />
                  </Button>
                  <Button
                    type="button"
                    size="icon-sm"
                    variant="ghost"
                    onClick={() => void handleDelete(template)}
                    disabled={isBusy || template.builtin}
                    aria-label={`Delete ${template.name}`}
                  >
                    <Trash2 className="size-4" />
                  </Button>
                </ItemActions>
              </Item>
            ))}
          </ItemGroup>
        )}
      </SettingBlock>

      <SettingBlock
        title="Text templates"
        description="Edit Go text/template bodies for clipboard copy. Duplicate the builtin summary to customize — originals stay read-only."
      >
        <div className="flex justify-end">
          <Button
            type="button"
            size="sm"
            onClick={openCreateText}
            disabled={isBusy || !isShietAppAvailable()}
          >
            <Plus className="size-4" />
            New text template
          </Button>
        </div>

        {!isShietAppAvailable() ? (
          <p className="text-sm text-muted-foreground">
            Template editing requires the desktop app.
          </p>
        ) : templatesQuery.isLoading ? (
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <LoaderCircle className="size-4 animate-spin" />
            Loading templates
          </div>
        ) : textTemplates.length === 0 ? (
          <p className="text-sm text-muted-foreground">No text templates yet.</p>
        ) : (
          <ItemGroup className="gap-2">
            {textTemplates.map((template) => (
              <Item key={template.id} variant="outline">
                <ItemContent className="min-w-0">
                  <ItemTitle className="flex flex-wrap items-center gap-2">
                    {template.name}
                    {template.builtin ? (
                      <span className="rounded-full bg-muted px-2 py-0.5 text-[10px] font-medium text-muted-foreground">
                        Builtin
                      </span>
                    ) : null}
                    <span className="rounded-full bg-muted px-2 py-0.5 text-[10px] font-medium uppercase text-muted-foreground">
                      text
                    </span>
                  </ItemTitle>
                  <ItemDescription className="line-clamp-2 text-xs">
                    {template.description || template.key}
                  </ItemDescription>
                </ItemContent>
                <ItemActions className="flex items-center gap-1">
                  <Button
                    type="button"
                    size="icon-sm"
                    variant="ghost"
                    onClick={() => void handleDuplicate(template)}
                    disabled={isBusy}
                    aria-label={`Duplicate ${template.name}`}
                  >
                    <Copy className="size-4" />
                  </Button>
                  <Button
                    type="button"
                    size="icon-sm"
                    variant="ghost"
                    onClick={() => openEditText(template)}
                    disabled={isBusy || template.builtin}
                    aria-label={`Edit ${template.name}`}
                  >
                    <Pencil className="size-4" />
                  </Button>
                  <Button
                    type="button"
                    size="icon-sm"
                    variant="ghost"
                    onClick={() => void handleDelete(template)}
                    disabled={isBusy || template.builtin}
                    aria-label={`Delete ${template.name}`}
                  >
                    <Trash2 className="size-4" />
                  </Button>
                </ItemActions>
              </Item>
            ))}
          </ItemGroup>
        )}
      </SettingBlock>

      <Dialog open={editorOpen} onOpenChange={setEditorOpen}>
        <DialogContent className="max-h-[90vh] max-w-2xl overflow-y-auto">
          <DialogHeader>
            <DialogTitle>
              {editingTemplate ? "Edit template" : "New template"}
            </DialogTitle>
            <DialogDescription>
              Choose grain and layout, then pick columns. Preview uses the
              current period.
            </DialogDescription>
          </DialogHeader>

          <div className="grid gap-4">
            <div className="grid gap-1.5">
              <Label htmlFor="export-template-name" className="text-xs">
                Name
              </Label>
              <Input
                id="export-template-name"
                value={draft.name}
                onChange={(event) =>
                  setDraft((current) => ({
                    ...current,
                    name: event.target.value,
                  }))
                }
                placeholder="Client timesheet"
              />
            </div>

            <div className="grid gap-1.5">
              <Label htmlFor="export-template-description" className="text-xs">
                Description
              </Label>
              <Input
                id="export-template-description"
                value={draft.description}
                onChange={(event) =>
                  setDraft((current) => ({
                    ...current,
                    description: event.target.value,
                  }))
                }
                placeholder="Optional"
              />
            </div>

            <div className="grid gap-3 sm:grid-cols-3">
              <Field className="gap-1.5">
                <FieldLabel className="text-xs">Grain</FieldLabel>
                <Select
                  value={draft.spec.grain}
                  onValueChange={(value) => setGrain(value as ExportGrain)}
                >
                  <SelectTrigger className="w-full bg-background" size="sm">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="rollup">Rollup</SelectItem>
                    <SelectItem value="detail">Detail</SelectItem>
                  </SelectContent>
                </Select>
              </Field>

              <Field className="gap-1.5">
                <FieldLabel className="text-xs">Layout</FieldLabel>
                <Select
                  value={draft.spec.layout}
                  onValueChange={(value) => setLayout(value as ExportLayout)}
                  disabled={draft.spec.grain === "detail"}
                >
                  <SelectTrigger className="w-full bg-background" size="sm">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="flat">Flat</SelectItem>
                    <SelectItem value="matrix">Matrix</SelectItem>
                  </SelectContent>
                </Select>
              </Field>

              <Field className="gap-1.5">
                <FieldLabel className="text-xs">Delimiter</FieldLabel>
                <Select
                  value={draft.spec.delimiter === "\t" ? "tab" : "comma"}
                  onValueChange={(value) =>
                    setDraft((current) => ({
                      ...current,
                      spec: {
                        ...current.spec,
                        delimiter: value === "tab" ? "\t" : ",",
                      },
                    }))
                  }
                >
                  <SelectTrigger className="w-full bg-background" size="sm">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="comma">CSV (,)</SelectItem>
                    <SelectItem value="tab">TSV (tab)</SelectItem>
                  </SelectContent>
                </Select>
              </Field>
            </div>

            <div className="grid gap-2">
              <Label className="text-xs">Columns</Label>
              <div className="grid gap-2 rounded-lg border p-3">
                {catalog.map((field) => {
                  const selected = draft.spec.columns.some(
                    (column) => column.field === field.field,
                  );
                  return (
                    <label
                      key={field.field}
                      className="flex items-start gap-2 text-sm"
                    >
                      <Checkbox
                        checked={selected}
                        onCheckedChange={(checked) =>
                          toggleField(field.field, Boolean(checked))
                        }
                        className="mt-0.5"
                      />
                      <span className="min-w-0">
                        <span className="font-medium">{field.label}</span>
                        <span className="block text-xs text-muted-foreground">
                          {field.description}
                        </span>
                      </span>
                    </label>
                  );
                })}
              </div>
            </div>

            {draft.spec.columns.length > 0 ? (
              <div className="grid gap-2">
                <Label className="text-xs">Order & headers</Label>
                <div className="grid gap-2">
                  {draft.spec.columns.map((column, index) => (
                    <div
                      key={column.field}
                      className="flex items-center gap-2 rounded-lg border px-2 py-1.5"
                    >
                      <div className="flex flex-col">
                        <Button
                          type="button"
                          size="icon-sm"
                          variant="ghost"
                          disabled={index === 0}
                          onClick={() => moveColumn(index, -1)}
                          aria-label={`Move ${column.field} up`}
                        >
                          <ChevronUp className="size-3.5" />
                        </Button>
                        <Button
                          type="button"
                          size="icon-sm"
                          variant="ghost"
                          disabled={index === draft.spec.columns.length - 1}
                          onClick={() => moveColumn(index, 1)}
                          aria-label={`Move ${column.field} down`}
                        >
                          <ChevronDown className="size-3.5" />
                        </Button>
                      </div>
                      <div className="min-w-0 flex-1 grid gap-1">
                        <span className="text-[11px] text-muted-foreground">
                          {column.field}
                          {draft.spec.layout === "matrix" &&
                          column.field !== "total"
                            ? " · days auto-inserted after row labels"
                            : null}
                        </span>
                        <Input
                          value={column.header}
                          onChange={(event) =>
                            updateColumnHeader(index, event.target.value)
                          }
                          className="h-8"
                        />
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            ) : null}

            <div className="grid gap-1.5">
              <Label className="text-xs">Live preview</Label>
              {!period?.id ? (
                <p className="text-xs text-muted-foreground">
                  No current period to preview.
                </p>
              ) : previewQuery.isLoading ? (
                <div className="flex items-center gap-2 text-xs text-muted-foreground">
                  <LoaderCircle className="size-3.5 animate-spin" />
                  Rendering preview
                </div>
              ) : previewQuery.isError ? (
                <p className="text-xs text-destructive">
                  {previewQuery.error instanceof Error
                    ? previewQuery.error.message
                    : "Preview failed"}
                </p>
              ) : (
                <pre className="max-h-48 overflow-auto rounded-lg border bg-muted/40 p-3 text-[11px] leading-relaxed whitespace-pre-wrap">
                  {previewQuery.data?.content || "(empty)"}
                </pre>
              )}
            </div>

            {formError ? (
              <p className="flex items-center gap-2 text-xs text-destructive">
                <AlertCircle className="size-3.5" />
                {formError}
              </p>
            ) : null}
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="secondary"
              onClick={() => setEditorOpen(false)}
              disabled={isBusy}
            >
              Cancel
            </Button>
            <Button
              type="button"
              onClick={() => void handleSave()}
              disabled={isBusy}
            >
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

      <Dialog open={textEditorOpen} onOpenChange={setTextEditorOpen}>
        <DialogContent className="max-h-[90vh] max-w-2xl overflow-y-auto">
          <DialogHeader>
            <DialogTitle>
              {editingTextTemplate
                ? "Edit text template"
                : "New text template"}
            </DialogTitle>
            <DialogDescription>
              Write a Go text/template body. Helpers: duration, signedDuration,
              hoursPerDay. Preview uses the current period.
            </DialogDescription>
          </DialogHeader>

          <div className="grid gap-4">
            <div className="grid gap-1.5">
              <Label htmlFor="export-text-template-name" className="text-xs">
                Name
              </Label>
              <Input
                id="export-text-template-name"
                value={textDraft.name}
                onChange={(event) =>
                  setTextDraft((current) => ({
                    ...current,
                    name: event.target.value,
                  }))
                }
                placeholder="Client paste summary"
              />
            </div>

            <div className="grid gap-1.5">
              <Label
                htmlFor="export-text-template-description"
                className="text-xs"
              >
                Description
              </Label>
              <Input
                id="export-text-template-description"
                value={textDraft.description}
                onChange={(event) =>
                  setTextDraft((current) => ({
                    ...current,
                    description: event.target.value,
                  }))
                }
                placeholder="Optional"
              />
            </div>

            <div className="grid gap-1.5">
              <Label htmlFor="export-text-template-body" className="text-xs">
                Template body
              </Label>
              <textarea
                id="export-text-template-body"
                value={textDraft.body}
                onChange={(event) =>
                  setTextDraft((current) => ({
                    ...current,
                    body: event.target.value,
                  }))
                }
                rows={14}
                spellCheck={false}
                className="min-h-48 w-full rounded-lg border bg-background px-3 py-2 font-mono text-[11px] leading-relaxed outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
                placeholder="Period: {{.PeriodLabel}}"
              />
            </div>

            <div className="grid gap-1.5">
              <Label className="text-xs">Live preview</Label>
              {!period?.id ? (
                <p className="text-xs text-muted-foreground">
                  No current period to preview.
                </p>
              ) : !debouncedTextBody.trim() ? (
                <p className="text-xs text-muted-foreground">
                  Enter a template body to preview.
                </p>
              ) : textPreviewQuery.isLoading ? (
                <div className="flex items-center gap-2 text-xs text-muted-foreground">
                  <LoaderCircle className="size-3.5 animate-spin" />
                  Rendering preview
                </div>
              ) : textPreviewQuery.isError ? (
                <p className="text-xs text-destructive">
                  {textPreviewQuery.error instanceof Error
                    ? textPreviewQuery.error.message
                    : "Preview failed"}
                </p>
              ) : (
                <pre className="max-h-48 overflow-auto rounded-lg border bg-muted/40 p-3 text-[11px] leading-relaxed whitespace-pre-wrap">
                  {textPreviewQuery.data?.content || "(empty)"}
                </pre>
              )}
            </div>

            {textFormError ? (
              <p className="flex items-center gap-2 text-xs text-destructive">
                <AlertCircle className="size-3.5" />
                {textFormError}
              </p>
            ) : null}
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="secondary"
              onClick={() => setTextEditorOpen(false)}
              disabled={isBusy}
            >
              Cancel
            </Button>
            <Button
              type="button"
              onClick={() => void handleSaveText()}
              disabled={isBusy}
            >
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
