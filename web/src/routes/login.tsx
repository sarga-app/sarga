import { createRoute, redirect } from '@tanstack/react-router'
import { rootRoute } from './root'
import { LoginPage } from '@/features/auth/login-page'

export const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/login',
  beforeLoad: ({ context }) => {
    if (context.auth.user) {
      throw redirect({ to: '/library' })
    }
  },
  component: LoginPage,
})
