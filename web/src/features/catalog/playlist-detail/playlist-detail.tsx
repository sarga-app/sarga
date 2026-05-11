import { useQuery } from '@tanstack/react-query'
import { getPlaylist } from '@/services/catalog'
import { TrackRow } from '@/components/ui/track-row'
import { PageLoader } from '@/components/ui/page-loader'
import styles from '../album-detail/album-detail.module.css'
import type { PlaylistDetailProps } from './playlist-detail.types'

export function PlaylistDetail({ id, libraryId }: PlaylistDetailProps) {
  const { data, isLoading, isError } = useQuery({
    queryKey: ['playlist', id, libraryId],
    queryFn: () => getPlaylist(id, libraryId),
  })

  if (isLoading) return <PageLoader />

  if (isError || !data) {
    return (
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 'var(--s-10) 0' }}>
        <p style={{ color: 'var(--fg-2)', margin: 0 }}>
          {isError ? 'Failed to load playlist' : 'Playlist not found'}
        </p>
      </div>
    )
  }

  return (
    <div>
      <h1 className={styles.title} style={{ margin: '0 0 var(--s-2)' }}>{data.name}</h1>
      {data.description && (
        <p style={{ fontSize: 14, color: 'var(--fg-3)', margin: '0 0 var(--s-5)' }}>{data.description}</p>
      )}
      <p style={{ fontFamily: 'var(--font-mono)', fontSize: 12, color: 'var(--fg-3)', margin: '0 0 var(--s-5)' }}>
        {data.track_count} tracks
      </p>

      <div>
        {data.tracks.map((track) => (
          <TrackRow
            key={track.track_id}
            position={track.position}
            title={track.title}
            format={track.format}
            durationMs={track.duration_ms}
          />
        ))}
      </div>
    </div>
  )
}
