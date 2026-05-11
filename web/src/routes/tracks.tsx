import { lazy } from 'react'
import { createRoute, useNavigate } from '@tanstack/react-router'
import { authRoute } from './auth'
import { useLibraryStore } from '@/stores/library-store'

const TracksList = lazy(() =>
  import('@/features/catalog/tracks-list').then((m) => ({ default: m.TracksList })),
)

interface TracksSearch { page?: number }

function TracksPage() {
  const activeLibraryId = useLibraryStore((s) => s.activeLibraryId)
  const { page = 1 } = tracksRoute.useSearch()
  const navigate = useNavigate()
  if (!activeLibraryId) return null
  return (
    <TracksList
      libraryId={activeLibraryId}
      page={page}
      onPageChange={(p) => navigate({ to: '/tracks', search: { page: p } })}
    />
  )
}

export const tracksRoute = createRoute({
  getParentRoute: () => authRoute,
  path: '/tracks',
  validateSearch: (s: Record<string, unknown>): TracksSearch => ({
    page: typeof s.page === 'number' ? s.page : typeof s.page === 'string' ? parseInt(s.page) || 1 : undefined,
  }),
  component: TracksPage,
})
