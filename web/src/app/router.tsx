import { createRouter } from '@tanstack/react-router'
import { rootRoute } from '@/routes/root'
import { indexRoute } from '@/routes/index'
import { loginRoute } from '@/routes/login'
import { authRoute } from '@/routes/auth'
import { libraryRoute } from '@/routes/library'
import { queryClient } from './query-client'

const routeTree = rootRoute.addChildren([
  indexRoute,
  loginRoute,
  authRoute.addChildren([libraryRoute]),
])

export const router = createRouter({
  routeTree,
  context: {
    queryClient,
    auth: { user: null, accessToken: null },
  },
})

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}
