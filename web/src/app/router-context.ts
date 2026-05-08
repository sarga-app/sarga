import type { QueryClient } from '@tanstack/react-query'
import type { User } from '@/features/auth/types'

export interface RouterContext {
  queryClient: QueryClient
  auth: {
    user: User | null
    accessToken: string | null
  }
}
