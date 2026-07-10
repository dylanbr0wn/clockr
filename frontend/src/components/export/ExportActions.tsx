import { Copy, Download, LoaderCircle } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { ClipboardSetText } from "../../../wailsjs/runtime/runtime";
import { Button } from "@/components/ui/button";
import { Field, FieldLabel } from "@/components/ui/field";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  exportPeriodCSV,
  exportPeriodText,
  isShietAppAvailable,
  saveExportFile,
} from "@/lib/api/shietService";
import { useCategories, useExportTemplates, type ExportTemplate } from "@/lib/api";
import {
  defaultExportFilename,
  EXPORT_TEMPLATE_DETAIL_ENTRIES_CSV,
  EXPORT_TEMPLATE_FLAT_DAILY_CSV,
  EXPORT_TEMPLATE_MATRIX_CSV,
  EXPORT_TEMPLATE_TEXT_SUMMARY,
  formatDetailEntriesCSV,
  formatFlatDailyCSV,
  formatSummaryCSV,
  formatSummaryText,
  isTabularFormat,
  isTextFormat,
  type PeriodExportSummary,
} from "@/lib/export";
import type { ScheduleItem } from "@/lib/schedule/types";

const FALLBACK_CSV_TEMPLATES: ExportTemplate[] = [
  {
    id: 1,
    key: EXPORT_TEMPLATE_MATRIX_CSV,
    name: "Category × day matrix",
    description:
      "Category rows by day columns with decimal hours and a Total column.",
    format: "csv",
    builtin: true,
    body: "",
  },
  {
    id: 2,
    key: EXPORT_TEMPLATE_FLAT_DAILY_CSV,
    name: "Flat daily totals",
    description:
      "One row per category per day with date, name, key, and decimal hours.",
    format: "csv",
    builtin: true,
    body: "",
  },
  {
    id: 3,
    key: EXPORT_TEMPLATE_DETAIL_ENTRIES_CSV,
    name: "Detail entries",
    description:
      "One row per event or gap fill with start, end, category, duration, and title.",
    format: "csv",
    builtin: true,
    body: "",
  },
];

const FALLBACK_TEXT_TEMPLATES: ExportTemplate[] = [
  {
    id: 4,
    key: EXPORT_TEMPLATE_TEXT_SUMMARY,
    name: "Text summary",
    description: "Human-readable period summary for clipboard copy.",
    format: "text",
    builtin: true,
    body: "",
  },
];

interface ExportActionsProps {
  summary: PeriodExportSummary | null;
  items?: ScheduleItem[];
  periodId?: number | null;
  disabled?: boolean;
  layout?: "inline" | "stacked";
}

export function ExportActions({
  summary,
  items = [],
  periodId = null,
  disabled = false,
  layout = "inline",
}: ExportActionsProps) {
  const [pendingAction, setPendingAction] = useState<"copy" | "csv" | null>(
    null,
  );
  const [csvTemplateKey, setCsvTemplateKey] = useState(
    EXPORT_TEMPLATE_MATRIX_CSV,
  );
  const [textTemplateKey, setTextTemplateKey] = useState(
    EXPORT_TEMPLATE_TEXT_SUMMARY,
  );
  const templatesQuery = useExportTemplates();
  const categoriesQuery = useCategories();

  const csvTemplates = useMemo(() => {
    const fromBackend = (templatesQuery.data ?? []).filter((template) =>
      isTabularFormat(template.format),
    );
    return fromBackend.length > 0 ? fromBackend : FALLBACK_CSV_TEMPLATES;
  }, [templatesQuery.data]);

  const textTemplates = useMemo(() => {
    const fromBackend = (templatesQuery.data ?? []).filter((template) =>
      isTextFormat(template.format),
    );
    return fromBackend.length > 0 ? fromBackend : FALLBACK_TEXT_TEMPLATES;
  }, [templatesQuery.data]);

  useEffect(() => {
    if (csvTemplates.some((template) => template.key === csvTemplateKey)) {
      return;
    }
    const first = csvTemplates[0];
    if (first) {
      setCsvTemplateKey(first.key);
    }
  }, [csvTemplates, csvTemplateKey]);

  useEffect(() => {
    if (textTemplates.some((template) => template.key === textTemplateKey)) {
      return;
    }
    const first = textTemplates[0];
    if (first) {
      setTextTemplateKey(first.key);
    }
  }, [textTemplates, textTemplateKey]);

  const categoryKeys = useMemo(() => {
    const keys: Record<string, string> = {};
    for (const category of categoriesQuery.data ?? []) {
      keys[category.name] = category.key || category.name;
    }
    return keys;
  }, [categoriesQuery.data]);

  const isBusy = pendingAction !== null;
  const buttonClassName =
    layout === "stacked" ? "w-full justify-center" : "flex-1";
  const canCopy =
    Boolean(periodId) && (isShietAppAvailable() || Boolean(summary));
  const canExportCSV =
    Boolean(periodId) && (isShietAppAvailable() || Boolean(summary));
  const showTextTemplatePicker = textTemplates.length > 1;

  const handleCopySummary = async () => {
    if (!periodId) {
      return;
    }

    setPendingAction("copy");
    try {
      let text: string;
      if (isShietAppAvailable()) {
        text = await exportPeriodText(periodId, textTemplateKey);
      } else if (summary) {
        // Browser-only fallback (Vite without Wails).
        text = formatSummaryText(summary);
      } else {
        return;
      }
      await ClipboardSetText(text);
      toast.success("Summary copied", {
        description: "Period summary is ready to paste into your timesheet.",
      });
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "Unable to copy summary";
      toast.error("Copy failed", { description: message });
    } finally {
      setPendingAction(null);
    }
  };

  const handleSaveCSV = async () => {
    if (!periodId) {
      return;
    }

    setPendingAction("csv");
    try {
      let savedPath: string;
      if (isShietAppAvailable()) {
        savedPath = await exportPeriodCSV(periodId, csvTemplateKey);
      } else if (summary) {
        // Browser-only fallback (Vite without Wails): keep local download working.
        let content: string;
        if (csvTemplateKey === EXPORT_TEMPLATE_FLAT_DAILY_CSV) {
          content = formatFlatDailyCSV(summary, categoryKeys);
        } else if (csvTemplateKey === EXPORT_TEMPLATE_DETAIL_ENTRIES_CSV) {
          content = formatDetailEntriesCSV(items, categoryKeys);
        } else {
          content = formatSummaryCSV(summary);
        }
        savedPath = await saveExportFile(
          defaultExportFilename(summary),
          content,
        );
      } else {
        return;
      }

      if (savedPath) {
        toast.success("CSV exported", { description: savedPath });
      }
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "Unable to export CSV";
      toast.error("Export failed", { description: message });
    } finally {
      setPendingAction(null);
    }
  };

  return (
    <div
      className={
        layout === "stacked" ? "grid gap-2" : "flex flex-col gap-2"
      }
    >
      {showTextTemplatePicker ? (
        <Field className="gap-1.5">
          <FieldLabel htmlFor="export-text-template" className="text-xs">
            Text template
          </FieldLabel>
          <Select
            value={textTemplateKey}
            onValueChange={setTextTemplateKey}
            disabled={disabled || isBusy || textTemplates.length === 0}
          >
            <SelectTrigger
              id="export-text-template"
              className="w-full bg-background"
              size="sm"
            >
              <SelectValue placeholder="Choose text template" />
            </SelectTrigger>
            <SelectContent>
              {textTemplates.map((template) => (
                <SelectItem key={template.key} value={template.key}>
                  {template.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </Field>
      ) : null}
      <Field className="gap-1.5">
        <FieldLabel htmlFor="export-csv-template" className="text-xs">
          CSV template
        </FieldLabel>
        <Select
          value={csvTemplateKey}
          onValueChange={setCsvTemplateKey}
          disabled={disabled || isBusy || csvTemplates.length === 0}
        >
          <SelectTrigger
            id="export-csv-template"
            className="w-full bg-background"
            size="sm"
          >
            <SelectValue placeholder="Choose template" />
          </SelectTrigger>
          <SelectContent>
            {csvTemplates.map((template) => (
              <SelectItem key={template.key} value={template.key}>
                {template.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </Field>
      <div
        className={
          layout === "stacked" ? "grid gap-2" : "flex items-center gap-2"
        }
      >
        <Button
          type="button"
          variant="outline"
          size="sm"
          className={buttonClassName}
          disabled={disabled || !canCopy || isBusy}
          onClick={() => void handleCopySummary()}
        >
          {pendingAction === "copy" ? (
            <LoaderCircle className="size-4 animate-spin" />
          ) : (
            <Copy className="size-4" />
          )}
          Copy
        </Button>
        <Button
          type="button"
          variant="outline"
          size="sm"
          className={buttonClassName}
          disabled={disabled || !canExportCSV || isBusy}
          onClick={() => void handleSaveCSV()}
        >
          {pendingAction === "csv" ? (
            <LoaderCircle className="size-4 animate-spin" />
          ) : (
            <Download className="size-4" />
          )}
          Export CSV
        </Button>
      </div>
    </div>
  );
}
