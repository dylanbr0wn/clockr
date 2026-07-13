import {
  Card,
  CardContent,
  CardHeader,
} from "@/components/ui/card";
import { categoriesForAssignPicker } from "@/lib/schedule";
import { EventEditDialog } from "./EventEditDialog";
import { GapSuggestDialog } from "./GapSuggestDialog";
import { ReviewQueueDialog } from "./ReviewQueueDialog";
import { ScheduleHeader } from "./ScheduleHeader";
import { ScheduleSidebar } from "./ScheduleSidebar";
import { ScheduleTimeline } from "./ScheduleTimeline";
import { useSchedulePage } from "./useSchedulePage";

export function SchedulePage() {
  const schedule = useSchedulePage();
  const editPickerCategories = categoriesForAssignPicker(
    schedule.categories,
    schedule.editingEvent?.categoryId,
  );
  const gapPickerCategories = categoriesForAssignPicker(schedule.categories);

  return (
    <>
      <section className="grid min-h-0 h-full flex-1 gap-3 overflow-hidden bg-background lg:grid-cols-[minmax(0,1fr)_320px] pt-px pr-1">
        <Card className="app-no-drag flex min-h-0 flex-col gap-0 ml-3 mb-3 py-0">
          <CardHeader className="flex shrink-0 flex-row items-center justify-end gap-2 border-b py-2 [.border-b]:pb-2">
            <ScheduleHeader schedule={schedule} />
          </CardHeader>
          <CardContent className="flex min-h-0 flex-1 flex-col overflow-hidden py-0! px-0! bg-background-lighter">
            <ScheduleTimeline
              data={{
                days: schedule.days,
                items: schedule.items,
                allDayChipsByDay: schedule.allDayChipsByDay,
                visibleGaps: schedule.visibleGaps,
                resettableDays: schedule.resettableDays,
                visibleDayCount: schedule.visibleDayCount,
                aiConfigured: schedule.aiConfigured,
              }}
              actions={{
                onCreate: schedule.handleCreate,
                onPreviewChange: schedule.setPreview,
                onCommitChange: schedule.handleCommit,
                onEditItem: schedule.handleOpenEventEditor,
                onDuplicateItem: schedule.handleDuplicateEvent,
                onRemoveItem: schedule.handleRemoveEvent,
                onExcludeItem: schedule.handleExcludeEvent,
                onExcludeAllDayChip: schedule.handleExcludeAllDayChip,
                onResetDay: schedule.handleResetDay,
                onSelectGap: schedule.handleSelectGap,
                onOpenReviewQueue: () => schedule.setReviewQueueOpen(true),
              }}
            />
          </CardContent>
        </Card>
        <ScheduleSidebar
          activePeriod={schedule.activePeriod}
          categories={schedule.categories}
          items={schedule.items}
          events={schedule.events}
          reviewDecisions={schedule.reviewDecisions}
          visibleGaps={schedule.visibleGaps}
          onOpenReviewQueue={() => schedule.setReviewQueueOpen(true)}
          isBackendLoading={schedule.isBackendLoading}
          backendError={schedule.backendError}
        />
      </section>
      <ReviewQueueDialog
        open={schedule.reviewQueueOpen}
        periodId={schedule.activePeriodId}
        onOpenChange={schedule.setReviewQueueOpen}
      />
      <EventEditDialog
        categories={editPickerCategories}
        event={schedule.editingEvent}
        isSaving={schedule.editEventPending || schedule.createPending}
        open={schedule.editingEvent !== null}
        onOpenChange={(open) => {
          if (!open) {
            schedule.handleCloseEventEditor();
          }
        }}
        onSave={schedule.handleSaveEventEdit}
      />
      <GapSuggestDialog
        aiConfigured={schedule.aiConfigured}
        categories={gapPickerCategories}
        evidenceError={schedule.gapEvidenceError}
        evidenceItems={schedule.gapEvidenceItems}
        evidencePending={schedule.gapEvidencePending}
        gap={schedule.selectedGap}
        isSaving={schedule.gapSuggestSaving}
        isSuggesting={schedule.gapSuggestPending}
        open={schedule.gapSuggestOpen}
        suggestError={schedule.gapSuggestError}
        suggestion={schedule.gapSuggestion}
        onConfirm={schedule.handleConfirmGapSuggest}
        onOpenChange={(open) => {
          if (!open) {
            schedule.handleCloseGapSuggest();
          }
        }}
        onRetrySuggest={schedule.handleRetryGapSuggest}
      />
    </>
  );
}
