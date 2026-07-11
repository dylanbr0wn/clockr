import { SchedulePage } from '@/components/schedule/SchedulePage'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/')({
  component: SchedulePage,
})
