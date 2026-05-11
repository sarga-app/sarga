import { createRoute } from '@tanstack/react-router'
import { Radio } from 'lucide-react'
import { authRoute } from './auth'
import { ComingSoon } from '@/components/ui/empty-state'

export const radioRoute = createRoute({
  getParentRoute: () => authRoute,
  path: '/radio',
  component: () => (
    <ComingSoon icon={Radio} title="Radio" body="Coming soon — smart radio and endless playback from your library." />
  ),
})
