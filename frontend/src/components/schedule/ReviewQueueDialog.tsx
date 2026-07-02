import { useMemo } from "react";
import { AlertCircleIcon } from "lucide-react";
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
  useEvents,
  useOpenReviewItems,
  useResolveReviewItem,
  type ReviewItem,
} from "@/lib/api";
import { errorMessage } from "@/lib/schedule";
import { buildReviewItemViews } from "./reviewQueue";

interface ReviewQueueDialogProps {
  periodId: number | undefined;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

function kindBadgeClass(kind: ReviewItem["kind"]) {
  switch (kind) {
    case "deleted_categorized":
      return "bg-destructive/10 text-destructive";
    case "new_in_gap":
      return "bg-amber-500/10 text-amber-700 dark:text-amber-300";
    case "title_changed":
      return "bg-blue-500/10 text-blue-700 dark:text-blue-300";
    case "tentative":
      return "bg-violet-500/10 text-violet-700 dark:text-violet-300";
    case "all_day":
      return "bg-slate-500/10 text-slate-700 dark:text-slate-300";
    default:
      return "bg-muted text-muted-foreground";
  }
}

export function ReviewQueueDialog({
  periodId,
  open,
  onOpenChange,
}: ReviewQueueDialogProps) {
  const reviewItemsQuery = useOpenReviewItems(periodId);
  const eventsQuery = useEvents(periodId);
  const resolveMutation = useResolveReviewItem();

  const views = useMemo(
    () =>
      buildReviewItemViews(
        reviewItemsQuery.data ?? [],
        eventsQuery.data ?? [],
      ),
    [eventsQuery.data, reviewItemsQuery.data],
  );

  const pendingId =
    resolveMutation.isPending && resolveMutation.variables
      ? resolveMutation.variables.reviewItemId
      : null;

  const handleResolve = (reviewItemId: number, action: string) => {
    resolveMutation.mutate(
      { reviewItemId, action },
      {
        onSuccess: () => {
          const remaining = (reviewItemsQuery.data?.length ?? 1) - 1;
          if (remaining <= 0) {
            onOpenChange(false);
          }
        },
      },
    );
  };

  const isLoading = reviewItemsQuery.isLoading || eventsQuery.isLoading;
  const error = reviewItemsQuery.error ?? eventsQuery.error ?? resolveMutation.error;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85vh] max-w-2xl overflow-hidden p-0">
        <DialogHeader className="border-b border-border px-6 py-4">
          <DialogTitle>Review queue</DialogTitle>
          <DialogDescription>
            {views.length > 0
              ? `${views.length} item${views.length === 1 ? "" : "s"} need a decision after the last sync. Safe changes were applied automatically.`
              : "No open review items for this period."}
          </DialogDescription>
        </DialogHeader>

        <div className="max-h-[55vh] space-y-3 overflow-y-auto px-6 py-4">
          {isLoading ? (
            <p className="text-sm text-muted-foreground">Loading review items…</p>
          ) : null}

          {!isLoading && views.length === 0 ? (
            <div className="flex items-center gap-3 rounded-lg border border-border bg-muted/40 p-4 text-sm text-muted-foreground">
              <AlertCircleIcon className="size-4 shrink-0" />
              <span>Everything is resolved. Sync again if your calendar changes.</span>
            </div>
          ) : null}

          {views.map((item) => (
            <div
              key={item.id}
              className="rounded-lg border border-border bg-card p-4 shadow-xs"
            >
              <div className="flex flex-wrap items-center gap-2">
                <span
                  className={`rounded-full px-2 py-0.5 text-xs font-medium ${kindBadgeClass(item.kind)}`}
                >
                  {item.tag}
                </span>
                <span className="font-medium text-foreground">{item.title}</span>
              </div>
              <p className="mt-2 text-sm text-muted-foreground">{item.description}</p>
              <div className="mt-4 flex flex-wrap gap-2">
                {item.secondaryAction ? (
                  <Button
                    disabled={pendingId === item.id}
                    size="sm"
                    variant="outline"
                    onClick={() =>
                      handleResolve(item.id, item.secondaryAction!.action)
                    }
                  >
                    {item.secondaryAction.label}
                  </Button>
                ) : null}
                <Button
                  disabled={pendingId === item.id}
                  size="sm"
                  onClick={() =>
                    handleResolve(item.id, item.primaryAction.action)
                  }
                >
                  {item.primaryAction.label}
                </Button>
              </div>
            </div>
          ))}

          {error ? (
            <p className="rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">
              {errorMessage(error)}
            </p>
          ) : null}
        </div>

        <DialogFooter className="border-t border-border px-6 py-4">
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            Close
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
