import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { RouterProvider } from '@tanstack/react-router'
import { router } from '@/app/router'
import { useAuthStore } from '@/features/auth/auth-store'
import '@/styles/global.css'

function AppRoot() {
  const { user, accessToken } = useAuthStore()
  return <RouterProvider router={router} context={{ auth: { user, accessToken }, queryClient: router.options.context.queryClient }} />
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <AppRoot />
  </StrictMode>,
)
