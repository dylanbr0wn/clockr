import { useEffect, useState, type FormEvent } from "react";
import { LoaderCircle, SparklesIcon } from "lucide-react";
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
  Field,
  FieldError,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field";
import {
  Item,
  ItemActions,
  ItemContent,
  ItemDescription,
  ItemMedia,
} from "@/components/ui/item";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { Category, GapSuggestion } from "@/lib/api";
import { formatMinutes } from "@/lib/scheduler";
import { errorMessage } from "@/lib/schedule";

const UNASSIGNED_CATEGORY_VALUE = "__unassigned__";

export interface SelectedGap {
  day: string;
  startMinutes: number;
  endMinutes: number;
  gapWindowStart: string;
  gapWindowEnd: string;
}

export interface GapSuggestConfirmValues {
  categoryId?: number;
  description: string;
}

interface GapSuggestDialogProps {
  gap: SelectedGap | null;
  categories: Category[];
  aiConfigured: boolean;
  aiLocal: boolean;
  open: boolean;
  isSuggesting: boolean;
  isSaving: boolean;
  suggestion: GapSuggestion | null;
  suggestError: unknown;
  onOpenChange: (open: boolean) => void;
  onRetrySuggest: () => void;
  onConfirm: (values: GapSuggestConfirmValues) => void;
}

export function GapSuggestDialog({
  gap,
  categories,
  aiConfigured,
  aiLocal,
  open,
  isSuggesting,
  isSaving,
  suggestion,
  suggestError,
  onOpenChange,
  onRetrySuggest,
  onConfirm,
}: GapSuggestDialogProps) {
  const [categoryValue, setCategoryValue] = useState(UNASSIGNED_CATEGORY_VALUE);
  const [description, setDescription] = useState("");
  const [formError, setFormError] = useState<string | null>(null);

  useEffect(() => {
    if (!open || !suggestion) {
      return;
    }

    const matched = categories.find(
      (category) => category.name === suggestion.category,
    );
    setCategoryValue(
      matched ? matched.id.toString() : UNASSIGNED_CATEGORY_VALUE,
    );
    setDescription(suggestion.description);
    setFormError(null);
  }, [categories, open, suggestion]);

  useEffect(() => {
    if (!open) {
      setCategoryValue(UNASSIGNED_CATEGORY_VALUE);
      setDescription("");
      setFormError(null);
    }
  }, [open]);

  const handleSubmit = (submitEvent: FormEvent<HTMLFormElement>) => {
    submitEvent.preventDefault();

    if (categoryValue === UNASSIGNED_CATEGORY_VALUE) {
      setFormError("Choose a category before saving.");
      return;
    }

    onConfirm({
      categoryId: Number(categoryValue),
      description: description.trim(),
    });
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Fill gap with AI</DialogTitle>
          <DialogDescription>
            {gap
              ? `${gap.day} · ${formatMinutes(gap.startMinutes)}-${formatMinutes(
                  gap.endMinutes,
                )}`
              : "Suggest a category for this uncovered interval."}
          </DialogDescription>
        </DialogHeader>

        {!aiConfigured ? (
          <Item variant="muted">
            <ItemContent>
              <ItemDescription>AI is not configured yet.</ItemDescription>
              <ItemDescription>
                Open Settings → AI Model and connect a local endpoint (e.g. LM
                Studio) or cloud provider before requesting suggestions.
              </ItemDescription>
            </ItemContent>
          </Item>
        ) : isSuggesting ? (
          <Item variant="muted">
            <ItemMedia variant="icon">
              <LoaderCircle className="animate-spin" />
            </ItemMedia>
            <ItemContent>
              <ItemDescription>
                Asking the model for a suggestion…
              </ItemDescription>
            </ItemContent>
          </Item>
        ) : suggestError ? (
          <div className="flex flex-col gap-3">
            <Item
              variant="outline"
              size="sm"
              className="border-destructive/30 bg-destructive/10"
            >
              <ItemContent>
                <ItemDescription className="text-destructive">
                  {errorMessage(suggestError)}
                </ItemDescription>
              </ItemContent>
            </Item>
            <ItemActions>
              <Button type="button" variant="outline" onClick={onRetrySuggest}>
                <SparklesIcon data-icon="inline-start" />
                Retry suggestion
              </Button>
            </ItemActions>
          </div>
        ) : (
          <form className="grid gap-4" onSubmit={handleSubmit}>
            {suggestion && aiLocal && suggestion.evidenceCount > 0 && (
              <Item variant="muted" size="sm">
                <ItemContent>
                  <ItemDescription>
                    Based on {suggestion.evidenceCount} local activity item
                    {suggestion.evidenceCount === 1 ? "" : "s"} in this
                    interval.
                  </ItemDescription>
                </ItemContent>
              </Item>
            )}

            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="gap-suggest-description">Description</FieldLabel>
                <Input
                  id="gap-suggest-description"
                  value={description}
                  onChange={(changeEvent) =>
                    setDescription(changeEvent.target.value)
                  }
                  placeholder="What were you working on?"
                />
              </Field>

              <Field>
                <FieldLabel htmlFor="gap-suggest-category">Category</FieldLabel>
                <Select value={categoryValue} onValueChange={setCategoryValue}>
                  <SelectTrigger id="gap-suggest-category" className="w-full">
                    <SelectValue placeholder="Choose category" />
                  </SelectTrigger>
                  <SelectContent position="popper" align="start">
                    <SelectItem value={UNASSIGNED_CATEGORY_VALUE}>
                      Unassigned
                    </SelectItem>
                    {categories.map((category) => (
                      <SelectItem
                        key={category.id}
                        value={category.id.toString()}
                      >
                        {category.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </Field>

              {formError ? <FieldError>{formError}</FieldError> : null}
            </FieldGroup>

            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                onClick={() => onOpenChange(false)}
              >
                Cancel
              </Button>
              <Button type="submit" disabled={isSaving || !suggestion}>
                {isSaving ? "Saving" : "Accept"}
              </Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  );
}
