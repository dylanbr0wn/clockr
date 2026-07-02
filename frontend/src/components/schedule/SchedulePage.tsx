import { Separator } from "@/components/ui/separator";
import { EventEditDialog } from "./EventEditDialog";
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
          resettableDays={schedule.resettableDays}
          visibleDayCount={schedule.visibleDayCount}
          onCreate={schedule.handleCreate}
          onPreviewChange={schedule.setPreview}
          onCommitChange={schedule.handleCommit}
          onEditItem={schedule.handleOpenEventEditor}
          onDuplicateItem={schedule.handleDuplicateEvent}
          onRemoveItem={schedule.handleRemoveEvent}
          onResetDay={schedule.handleResetDay}
        />
        <ScheduleSidebar
          activePeriod={schedule.activePeriod}
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
    </>
  );
}
