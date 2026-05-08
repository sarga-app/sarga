import { useMutation } from '@tanstack/react-query'
import { useRouter } from '@tanstack/react-router'
import { logout } from './auth-service'
import { useAuthStore } from './auth-store'

export function useLogout() {
  const logoutStore = useAuthStore((s) => s.logout)
  const router = useRouter()

  return useMutation({
    mutationFn: logout,
    onSettled: () => {
      logoutStore()
      void router.navigate({ to: '/login' })
    },
  })
}
