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
  Item,
  ItemContent,
  ItemDescription,
  ItemFooter,
  ItemGroup,
  ItemMedia,
  ItemTitle,
} from "@/components/ui/item";
import {
  useReviewDecisions,
  useResolveReviewDecision,
  type ReviewDecision,
  type ReviewDecisionAction,
} from "@/lib/api";
import { errorMessage } from "@/lib/schedule";
import { kindBadgeClass } from "./reviewQueue";

interface ReviewQueueDialogProps {
  periodId: number | undefined;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

function actionVariant(action: ReviewDecisionAction) {
  return action.variant ?? (action.role === "primary" ? "default" : "outline");
}

export function ReviewQueueDialog({
  periodId,
  open,
  onOpenChange,
}: ReviewQueueDialogProps) {
  const reviewDecisionsQuery = useReviewDecisions(periodId);
  const resolveMutation = useResolveReviewDecision();

  const decisions = useMemo(
    () => reviewDecisionsQuery.data ?? [],
    [reviewDecisionsQuery.data],
  );

  const pendingId =
    resolveMutation.isPending && resolveMutation.variables
      ? resolveMutation.variables.decisionId
      : null;

  const handleResolve = (decisionId: number, action: string) => {
    resolveMutation.mutate(
      { decisionId, action },
      {
        onSuccess: () => {
          const remaining = (reviewDecisionsQuery.data?.length ?? 1) - 1;
          if (remaining <= 0) {
            onOpenChange(false);
          }
        },
      },
    );
  };

  const isLoading = reviewDecisionsQuery.isLoading;
  const error = reviewDecisionsQuery.error ?? resolveMutation.error;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85vh] max-w-2xl overflow-hidden p-0">
        <DialogHeader className="border-b border-border px-6 py-4">
          <DialogTitle>Review queue</DialogTitle>
          <DialogDescription>
            {decisions.length > 0
              ? `${decisions.length} item${decisions.length === 1 ? " needs" : "s need"} a decision after the last sync. Safe changes were applied automatically.`
              : "No open review items for this period."}
          </DialogDescription>
        </DialogHeader>

        <div className="max-h-[55vh] overflow-y-auto px-6 py-4">
          {isLoading ? (
            <p className="text-sm text-muted-foreground">Loading review items…</p>
          ) : null}

          {!isLoading && decisions.length === 0 ? (
            <ItemGroup>
              <Item variant="muted">
                <ItemMedia variant="icon">
                  <AlertCircleIcon />
                </ItemMedia>
                <ItemContent>
                  <ItemDescription>
                    Everything is resolved. Sync again if your calendar changes.
                  </ItemDescription>
                </ItemContent>
              </Item>
            </ItemGroup>
          ) : null}

          {decisions.length > 0 ? (
            <ItemGroup className="gap-3">
              {decisions.map((decision: ReviewDecision) => (
                <Item key={decision.id} variant="outline">
                  <ItemContent>
                    <ItemTitle className="flex flex-wrap items-center gap-2">
                      <span
                        className={`rounded-full px-2 py-0.5 text-xs font-medium ${kindBadgeClass(decision.kind)}`}
                      >
                        {decision.tag}
                      </span>
                      {decision.title}
                    </ItemTitle>
                    <ItemDescription>{decision.description}</ItemDescription>
                  </ItemContent>
                  <ItemFooter>
                    <div className="flex flex-wrap gap-2">
                      {decision.actions.map((action) => (
                        <Button
                          key={action.key}
                          disabled={pendingId === decision.id}
                          size="sm"
                          variant={actionVariant(action)}
                          onClick={() => handleResolve(decision.id, action.key)}
                        >
                          {action.label}
                        </Button>
                      ))}
                    </div>
                  </ItemFooter>
                </Item>
              ))}
            </ItemGroup>
          ) : null}

          {error ? (
            <Item
              variant="outline"
              size="sm"
              className="mt-3 border-destructive/30 bg-destructive/10"
            >
              <ItemContent>
                <ItemDescription className="text-destructive">
                  {errorMessage(error)}
                </ItemDescription>
              </ItemContent>
            </Item>
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
