import type { components } from '@/types/api'

export type Album = components['schemas']['Album']
export type Artist = components['schemas']['Artist']
export type Track = components['schemas']['Track']

export interface SearchResultsProps {
  libraryId: string
  query: string
}
