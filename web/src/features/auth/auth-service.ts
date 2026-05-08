import { api } from '@/services/api'

export async function login(username: string, password: string) {
  const { data, error } = await api.POST('/auth/login', {
    body: { username, password },
  })
  if (error) {
    const msg = (error as { error?: { message?: string } }).error?.message
    throw new Error(msg ?? 'Invalid credentials')
  }
  return data
}

export async function logout() {
  await api.POST('/auth/logout', {})
}

export async function refresh() {
  const { data, error } = await api.POST('/auth/refresh', {})
  if (error) throw new Error('Session expired')
  return data
}
