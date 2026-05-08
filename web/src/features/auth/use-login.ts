import { useMutation } from '@tanstack/react-query'
import { useRouter } from '@tanstack/react-router'
import { login } from './auth-service'
import { useAuthStore } from './auth-store'

export function useLogin() {
  const { setUser, setAccessToken, logout } = useAuthStore()
  const router = useRouter()

  return useMutation({
    mutationFn: (creds: { username: string; password: string }) =>
      login(creds.username, creds.password),
    onSuccess: (data, creds) => {
      setAccessToken(data.access_token)
      setUser({ username: creds.username, isAdmin: false })
      void router.navigate({ to: '/library' })
    },
    onError: () => {
      logout()
    },
  })
}
