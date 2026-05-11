import { lazy } from 'react'
import { createRoute, useNavigate } from '@tanstack/react-router'
import { authRoute } from './auth'
import { useLibraryStore } from '@/stores/library-store'

const ArtistsList = lazy(() =>
  import('@/features/catalog/artists-list').then((m) => ({ default: m.ArtistsList })),
)

interface ArtistsSearch { page?: number }

function ArtistsPage() {
  const activeLibraryId = useLibraryStore((s) => s.activeLibraryId)
  const { page = 1 } = artistsRoute.useSearch()
  const navigate = useNavigate()
  if (!activeLibraryId) return null
  return (
    <ArtistsList
      libraryId={activeLibraryId}
      page={page}
      onPageChange={(p) => navigate({ to: '/artists', search: { page: p } })}
    />
  )
}

export const artistsRoute = createRoute({
  getParentRoute: () => authRoute,
  path: '/artists',
  validateSearch: (s: Record<string, unknown>): ArtistsSearch => ({
    page: typeof s.page === 'number' ? s.page : typeof s.page === 'string' ? parseInt(s.page) || 1 : undefined,
  }),
  component: ArtistsPage,
})
