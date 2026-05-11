import { api, authApi } from '@/services/api'

export async function login(username: string, password: string) {
  const { data, error } = await authApi.POST('/auth/login', {
    body: { username, password },
  })
  if (error) {
    const msg = (error as { error?: { message?: string } }).error?.message
    throw new Error(msg ?? 'Invalid credentials')
  }
  return data
}

export async function me(accessToken: string) {
  const { data, error } = await api.GET('/me', {
    headers: { Authorization: `Bearer ${accessToken}` },
  })
  if (error) throw new Error('Failed to fetch user identity')
  return data
}

export async function logout() {
  await authApi.POST('/auth/logout', {})
}
