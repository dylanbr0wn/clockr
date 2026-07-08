import { Copy, Download, LoaderCircle } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { ClipboardSetText } from "../../../wailsjs/runtime/runtime";
import { Button } from "@/components/ui/button";
import { saveExportFile } from "@/lib/api/clockrService";
import {
  defaultExportFilename,
  formatSummaryCSV,
  formatSummaryText,
  type PeriodExportSummary,
} from "@/lib/export";

interface ExportActionsProps {
  summary: PeriodExportSummary | null;
  disabled?: boolean;
  layout?: "inline" | "stacked";
}

export function ExportActions({
  summary,
  disabled = false,
  layout = "inline",
}: ExportActionsProps) {
  const [pendingAction, setPendingAction] = useState<"copy" | "csv" | null>(
    null,
  );
  const isBusy = pendingAction !== null;
  const buttonClassName =
    layout === "stacked" ? "w-full justify-center" : "flex-1";

  const handleCopySummary = async () => {
    if (!summary) {
      return;
    }

    setPendingAction("copy");
    try {
      const text = formatSummaryText(summary);
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
    if (!summary) {
      return;
    }

    setPendingAction("csv");
    try {
      const csv = formatSummaryCSV(summary);
      const filename = defaultExportFilename(summary);
      const savedPath = await saveExportFile(filename, csv);

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
        layout === "stacked" ? "grid gap-2" : "flex items-center gap-2"
      }
    >
      <Button
        type="button"
        variant="outline"
        size="sm"
        className={buttonClassName}
        disabled={disabled || !summary || isBusy}
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
        disabled={disabled || !summary || isBusy}
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
  );
}
