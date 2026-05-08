import { createRootRouteWithContext, Outlet } from '@tanstack/react-router'
import { Providers } from '@/app/providers'
import type { RouterContext } from '@/app/router-context'

export const rootRoute = createRootRouteWithContext<RouterContext>()({
  component: () => (
    <Providers>
      <Outlet />
    </Providers>
  ),
})
