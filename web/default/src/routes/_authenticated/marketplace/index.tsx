import { createFileRoute } from '@tanstack/react-router'
import { Marketplace } from '@/features/marketplace'

export const Route = createFileRoute('/_authenticated/marketplace/')({
  component: Marketplace,
})
