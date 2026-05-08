import { useMutation } from '@tanstack/react-query'
import { useRouter } from '@tanstack/react-router'
import { login, me } from './auth-service'
import { useAuthStore } from './auth-store'

export function useLogin() {
  const { setUser, setAccessToken, logout } = useAuthStore()
  const router = useRouter()

  return useMutation({
    mutationFn: async (creds: { username: string; password: string }) => {
      const loginData = await login(creds.username, creds.password)
      const meData = await me(loginData.access_token)
      return { loginData, meData, username: creds.username }
    },
    onSuccess: ({ loginData, meData, username }) => {
      setAccessToken(loginData.access_token)
      setUser({
        id: meData.user_id,
        username,
        isAdmin: meData.is_admin,
      })
      void router.navigate({ to: '/library' })
    },
    onError: () => {
      logout()
    },
  })
}
