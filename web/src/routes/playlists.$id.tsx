import { lazy } from 'react'
import { createRoute } from '@tanstack/react-router'
import { authRoute } from './auth'
import { useLibraryStore } from '@/stores/library-store'

const PlaylistDetail = lazy(() =>
  import('@/features/catalog/playlist-detail').then((m) => ({ default: m.PlaylistDetail })),
)

function PlaylistDetailPage() {
  const { id } = playlistDetailRoute.useParams()
  const activeLibraryId = useLibraryStore((s) => s.activeLibraryId)
  if (!activeLibraryId) return null
  return <PlaylistDetail id={id} libraryId={activeLibraryId} />
}

export const playlistDetailRoute = createRoute({
  getParentRoute: () => authRoute,
  path: '/playlists/$id',
  component: PlaylistDetailPage,
})
