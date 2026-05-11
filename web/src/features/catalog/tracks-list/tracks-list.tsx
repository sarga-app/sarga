import { useQuery, keepPreviousData } from '@tanstack/react-query'
import { Music2 } from 'lucide-react'
import { listTracks } from '@/services/catalog'
import { Pagination } from '@/components/ui/pagination'
import { FormatBadge } from '@/components/ui/format-badge'
import { EmptyState, ErrorState } from '@/components/ui/empty-state'
import { PageLoader } from '@/components/ui/page-loader'
import { formatDuration } from '@/lib/format'
import { TRACKS_PAGE_SIZE } from '@/lib/constants'
import styles from './tracks-list.module.css'
import type { TracksListProps, Track } from './tracks-list.types'

export function TracksList({ libraryId, page, onPageChange }: TracksListProps) {
  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ['tracks', libraryId, page],
    queryFn: () => listTracks(libraryId, page, TRACKS_PAGE_SIZE),
    placeholderData: keepPreviousData,
  })

  const tracks = data?.data ?? []
  const pagination = data?.pagination

  if (isLoading) return <PageLoader />
  if (isError) return <ErrorState message="Failed to load tracks" onRetry={refetch} />
  if (tracks.length === 0) {
    return <EmptyState icon={Music2} title="No tracks yet" body="Scan your library to get started" />
  }

  return (
    <div>
      {pagination && pagination.total_pages > 1 && (
        <Pagination page={page} totalPages={pagination.total_pages} onChange={onPageChange} />
      )}

      <div className={styles.header}>
        <span className={styles.headerNum} />
        <span className={styles.headerTitle}>Title</span>
        <span className={styles.headerTime}>Time</span>
      </div>

      {(tracks as Track[]).map((track, i) => (
        <div key={track.id} className={styles.row}>
          <span className={styles.rowNum}>{(page - 1) * TRACKS_PAGE_SIZE + i + 1}</span>
          <div className={styles.rowTitle}>{track.title}</div>
          <FormatBadge format={track.format} highlight={track.is_lossless ?? false} />
          <span className={styles.rowDuration}>{formatDuration(track.duration_ms)}</span>
        </div>
      ))}
    </div>
  )
}
