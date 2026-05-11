import { lazy } from 'react'
import { createRoute } from '@tanstack/react-router'
import { authRoute } from './auth'
import { useLibraryStore } from '@/stores/library-store'

const ArtistDetail = lazy(() =>
  import('@/features/catalog/artist-detail').then((m) => ({ default: m.ArtistDetail })),
)

function ArtistDetailPage() {
  const { id } = artistDetailRoute.useParams()
  const activeLibraryId = useLibraryStore((s) => s.activeLibraryId)
  if (!activeLibraryId) return null
  return <ArtistDetail id={id} libraryId={activeLibraryId} />
}

export const artistDetailRoute = createRoute({
  getParentRoute: () => authRoute,
  path: '/artists/$id',
  component: ArtistDetailPage,
})
