import { Link } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { ListMusic } from 'lucide-react'
import { listPlaylists } from '@/services/catalog'
import { EmptyState, ErrorState } from '@/components/ui/empty-state'
import { PageLoader } from '@/components/ui/page-loader'
import styles from './playlists-list.module.css'
import type { PlaylistsListProps } from './playlists-list.types'

export function PlaylistsList({ libraryId }: PlaylistsListProps) {
  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ['playlists', libraryId],
    queryFn: () => listPlaylists(libraryId),
  })

  const playlists = data?.data ?? []

  if (isLoading) return <PageLoader />
  if (isError) return <ErrorState message="Failed to load playlists" onRetry={refetch} />
  if (playlists.length === 0) {
    return <EmptyState icon={ListMusic} title="No playlists yet" body="M3U playlists are imported during scanning" />
  }

  return (
    <div>
      {playlists.map((playlist) => (
        <Link
          key={playlist.id}
          to="/playlists/$id"
          params={{ id: playlist.id }}
          className={styles.row}
        >
          <span className={styles.rowName}>{playlist.name}</span>
          <span className={styles.rowCount}>{playlist.track_count} tracks</span>
        </Link>
      ))}
    </div>
  )
}
