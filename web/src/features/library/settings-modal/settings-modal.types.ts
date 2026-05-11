import type { components } from '@/types/api'

export type LibrarySettings = components['schemas']['LibrarySettings']
export type SettingsTab = 'scanning' | 'metadata' | 'artwork'

export interface SettingsModalProps {
  libraryId: string
  onClose: () => void
}

export interface PriorityListProps {
  items: string[]
  onChange: (items: string[]) => void
}
