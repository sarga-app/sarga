import { useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'
import { listLibraries } from '@/services/catalog'
import { useLibraryStore } from '@/stores/library-store'

/**
 * Single source of truth for library state. Owns:
 * - the libraries list query
 * - the auto-pick-first-library effect
 * - active library lookup with sane fallback
 */
export function useLibraries() {
  const { data, isLoading } = useQuery({ queryKey: ['libraries'], queryFn: listLibraries })
  const libraries = data?.libraries ?? []
  const { activeLibraryId, setActiveLibraryId } = useLibraryStore()

  useEffect(() => {
    if (!activeLibraryId && libraries.length > 0) {
      setActiveLibraryId(libraries[0].id)
    }
  }, [libraries, activeLibraryId, setActiveLibraryId])

  const activeLibrary =
    libraries.find((l) => l.id === activeLibraryId) ?? libraries[0] ?? null

  return { libraries, activeLibrary, activeLibraryId, setActiveLibraryId, isLoading }
}
