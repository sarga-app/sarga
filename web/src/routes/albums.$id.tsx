import { lazy } from 'react'
import { createRoute } from '@tanstack/react-router'
import { authRoute } from './auth'
import { useLibraryStore } from '@/stores/library-store'

const AlbumDetail = lazy(() =>
  import('@/features/catalog/album-detail').then((m) => ({ default: m.AlbumDetail })),
)

function AlbumDetailPage() {
  const { id } = albumDetailRoute.useParams()
  const activeLibraryId = useLibraryStore((s) => s.activeLibraryId)
  if (!activeLibraryId) return null
  return <AlbumDetail id={id} libraryId={activeLibraryId} />
}

export const albumDetailRoute = createRoute({
  getParentRoute: () => authRoute,
  path: '/albums/$id',
  component: AlbumDetailPage,
})
