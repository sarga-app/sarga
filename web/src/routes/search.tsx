import { lazy } from 'react'
import { createRoute } from '@tanstack/react-router'
import { authRoute } from './auth'
import { useLibraryStore } from '@/stores/library-store'

const SearchResults = lazy(() =>
  import('@/features/catalog/search-results').then((m) => ({ default: m.SearchResults })),
)

interface SearchParams { q?: string }

function SearchPage() {
  const { q = '' } = searchRoute.useSearch()
  const activeLibraryId = useLibraryStore((s) => s.activeLibraryId)
  if (!activeLibraryId) return null
  return <SearchResults libraryId={activeLibraryId} query={q} />
}

export const searchRoute = createRoute({
  getParentRoute: () => authRoute,
  path: '/search',
  validateSearch: (search: Record<string, unknown>): SearchParams => ({
    q: typeof search.q === 'string' ? search.q : undefined,
  }),
  component: SearchPage,
})
