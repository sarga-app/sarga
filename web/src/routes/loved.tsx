import { createRoute } from '@tanstack/react-router'
import { Heart } from 'lucide-react'
import { authRoute } from './auth'
import { ComingSoon } from '@/components/ui/empty-state'

export const lovedRoute = createRoute({
  getParentRoute: () => authRoute,
  path: '/loved',
  component: () => (
    <ComingSoon icon={Heart} title="Loved" body="Coming soon — your loved tracks and albums in one place." />
  ),
})
