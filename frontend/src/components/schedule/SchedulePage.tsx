import { Separator } from "@/components/ui/separator";
import { EventEditDialog } from "./EventEditDialog";
import { GapSuggestDialog } from "./GapSuggestDialog";
import { ReviewQueueDialog } from "./ReviewQueueDialog";
import { ScheduleHeader } from "./ScheduleHeader";
import { ScheduleSidebar } from "./ScheduleSidebar";
import { ScheduleTimeline } from "./ScheduleTimeline";
import { useSchedulePage } from "./useSchedulePage";

interface SchedulePageProps {
  titlebarPaddingClass: string;
}

export function SchedulePage({ titlebarPaddingClass }: SchedulePageProps) {
  const schedule = useSchedulePage();

  return (
    <>
      <ScheduleHeader
        titlebarPaddingClass={titlebarPaddingClass}
        schedule={schedule}
      />
      <Separator />
      <section className="grid min-h-0 flex-1 gap-4 lg:grid-cols-[minmax(0,1fr)_320px] p-3 bg-sidebar">
        <ScheduleTimeline
          days={schedule.days}
          items={schedule.items}
          allDayChipsByDay={schedule.allDayChipsByDay}
          visibleGaps={schedule.visibleGaps}
          resettableDays={schedule.resettableDays}
          visibleDayCount={schedule.visibleDayCount}
          aiConfigured={schedule.aiConfigured}
          onCreate={schedule.handleCreate}
          onPreviewChange={schedule.setPreview}
          onCommitChange={schedule.handleCommit}
          onEditItem={schedule.handleOpenEventEditor}
          onDuplicateItem={schedule.handleDuplicateEvent}
          onRemoveItem={schedule.handleRemoveEvent}
          onResetDay={schedule.handleResetDay}
          onSelectGap={schedule.handleSelectGap}
          onOpenReviewQueue={() => schedule.setReviewQueueOpen(true)}
        />
        <ScheduleSidebar
          activePeriod={schedule.activePeriod}
          items={schedule.items}
          visibleDayCount={schedule.visibleDayCount}
          totals={schedule.totals}
          preview={schedule.preview}
          counts={schedule.counts}
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
        categories={schedule.categories}
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
        aiLocal={schedule.aiLocal}
        categories={schedule.categories}
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
