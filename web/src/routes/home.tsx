import { createRoute } from '@tanstack/react-router'
import { Home } from 'lucide-react'
import { authRoute } from './auth'
import { ComingSoon } from '@/components/ui/empty-state'

export const homeRoute = createRoute({
  getParentRoute: () => authRoute,
  path: '/home',
  component: () => (
    <ComingSoon
      icon={Home}
      title="Home"
      body="Coming in Phase 3 — recently played, now playing, and your listening history."
    />
  ),
})
