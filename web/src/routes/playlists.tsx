import { lazy } from 'react'
import { createRoute } from '@tanstack/react-router'
import { authRoute } from './auth'
import { useLibraryStore } from '@/stores/library-store'

const PlaylistsList = lazy(() =>
  import('@/features/catalog/playlists-list').then((m) => ({ default: m.PlaylistsList })),
)

function PlaylistsPage() {
  const activeLibraryId = useLibraryStore((s) => s.activeLibraryId)
  if (!activeLibraryId) return null
  return <PlaylistsList libraryId={activeLibraryId} />
}

export const playlistsRoute = createRoute({
  getParentRoute: () => authRoute,
  path: '/playlists',
  component: PlaylistsPage,
})
