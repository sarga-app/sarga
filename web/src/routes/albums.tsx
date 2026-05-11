import { lazy } from 'react'
import { createRoute, useNavigate } from '@tanstack/react-router'
import { authRoute } from './auth'
import { useLibraryStore } from '@/stores/library-store'

const AlbumsGrid = lazy(() =>
  import('@/features/catalog/albums-grid').then((m) => ({ default: m.AlbumsGrid })),
)

type AlbumSort = 'date' | 'title' | 'added'

interface AlbumsSearch {
  page?: number
  sort?: AlbumSort
  compilations?: boolean
}

function parseSort(v: unknown): AlbumSort | undefined {
  if (v === 'date' || v === 'title' || v === 'added') return v
  return undefined
}

function AlbumsPage() {
  const activeLibraryId = useLibraryStore((s) => s.activeLibraryId)
  const { page = 1, sort = 'date', compilations = false } = albumsRoute.useSearch()
  const navigate = useNavigate()

  if (!activeLibraryId) return null

  return (
    <AlbumsGrid
      libraryId={activeLibraryId}
      page={page}
      sort={sort}
      compilations={compilations}
      onPageChange={(p) => navigate({ to: '/albums', search: (prev) => ({ ...prev, page: p }) })}
      onSortChange={(s) => navigate({ to: '/albums', search: (prev) => ({ ...prev, sort: s, page: 1 }) })}
      onCompilationsChange={(c) => navigate({ to: '/albums', search: (prev) => ({ ...prev, compilations: c, page: 1 }) })}
    />
  )
}

export const albumsRoute = createRoute({
  getParentRoute: () => authRoute,
  path: '/albums',
  validateSearch: (s: Record<string, unknown>): AlbumsSearch => ({
    page: typeof s.page === 'number' ? s.page : typeof s.page === 'string' ? parseInt(s.page) || 1 : undefined,
    sort: parseSort(s.sort),
    compilations: s.compilations === true || s.compilations === 'true' ? true : undefined,
  }),
  component: AlbumsPage,
})
