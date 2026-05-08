import { useEffect } from 'react'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { RouterProvider } from '@tanstack/react-router'
import { router } from '@/app/router'
import { useAuthStore } from '@/features/auth/auth-store'
import { useUIStore } from '@/stores/ui-store'
import '@/styles/global.css'

function AppRoot() {
  const { user, accessToken } = useAuthStore()
  const theme = useUIStore((s) => s.theme)

  useEffect(() => {
    const root = document.documentElement
    if (theme === 'nocturne') {
      root.classList.remove('surface-paper')
      root.classList.add('surface-nocturne', 'dark')
    } else {
      root.classList.remove('surface-nocturne', 'dark')
      root.classList.add('surface-paper')
    }
  }, [theme])

  return (
    <RouterProvider
      router={router}
      context={{
        auth: { user, accessToken },
        queryClient: router.options.context.queryClient,
      }}
    />
  )
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <AppRoot />
  </StrictMode>,
)
