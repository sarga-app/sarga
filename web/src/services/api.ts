import createClient from 'openapi-fetch'
import type { paths } from '@/types/api'
import { useAuthStore } from '@/features/auth/auth-store'

export const api = createClient<paths>({ baseUrl: '/api/v1', credentials: 'include' })

export const authApi = createClient<paths>({ baseUrl: '/api/v1', credentials: 'include' })

api.use({
  async onRequest({ request }) {
    const token = useAuthStore.getState().accessToken
    if (token) request.headers.set('Authorization', `Bearer ${token}`)
    return request
  },

  async onResponse({ response, request }) {
    if (response.status !== 401) return response
    try {
      const res = await authApi.POST('/auth/refresh', {})
      if (res.error) throw res.error
      const token = res.data!.access_token
      useAuthStore.getState().setAccessToken(token)
      request.headers.set('Authorization', `Bearer ${token}`)
      return fetch(request)
    } catch {
      useAuthStore.getState().logout()
      return response
    }
  },
})
