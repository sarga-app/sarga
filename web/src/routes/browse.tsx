import { createRoute } from '@tanstack/react-router'
import { BookOpen } from 'lucide-react'
import { authRoute } from './auth'
import { ComingSoon } from '@/components/ui/empty-state'

export const browseRoute = createRoute({
  getParentRoute: () => authRoute,
  path: '/browse',
  component: () => (
    <ComingSoon icon={BookOpen} title="Browse" body="Coming soon — discover new music across your library." />
  ),
})
