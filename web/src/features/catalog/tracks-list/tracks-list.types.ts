import type { components } from '@/types/api'

export type Track = components['schemas']['Track']

export interface TracksListProps {
  libraryId: string
  page: number
  onPageChange: (p: number) => void
}
