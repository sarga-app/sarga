import { createRoute, redirect } from '@tanstack/react-router'
import { rootRoute } from './root'
import { AppLayout } from '@/app/layout'

export const authRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: '_auth',
  beforeLoad: ({ context }) => {
    if (!context.auth.user) {
      throw redirect({ to: '/login' })
    }
  },
  component: AppLayout,
})
