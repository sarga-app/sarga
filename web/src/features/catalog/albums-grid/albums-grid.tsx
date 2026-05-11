import { useQuery, keepPreviousData } from '@tanstack/react-query'
import { Music } from 'lucide-react'
import { listAlbums } from '@/services/catalog'
import { Pagination } from '@/components/ui/pagination'
import { AlbumTile } from '@/components/ui/album-tile'
import { EmptyState, ErrorState } from '@/components/ui/empty-state'
import { PageLoader } from '@/components/ui/page-loader'
import { ALBUMS_PAGE_SIZE } from '@/lib/constants'
import { cn } from '@/lib/cn'
import styles from './albums-grid.module.css'
import type { AlbumSort, AlbumsGridProps } from './albums-grid.types'

const SORTS: { id: AlbumSort; label: string }[] = [
  { id: 'date', label: 'By Date' },
  { id: 'title', label: 'By Title' },
  { id: 'added', label: 'By Added' },
]

export function AlbumsGrid({
  libraryId, page, sort, compilations,
  onPageChange, onSortChange, onCompilationsChange,
}: AlbumsGridProps) {
  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ['albums', libraryId, sort, compilations, page],
    queryFn: () => listAlbums(libraryId, { sort, compilations_only: compilations, page, limit: ALBUMS_PAGE_SIZE }),
    placeholderData: keepPreviousData,
  })

  const albums = data?.data ?? []
  const pagination = data?.pagination
  const showLoader = isLoading

  return (
    <div>
      <div className={styles.headerRow}>
        <div className={styles.toolbar}>
          <div className={styles.sortGroup}>
            {SORTS.map(({ id, label }) => (
              <button
                key={id}
                onClick={() => onSortChange(id)}
                className={cn(styles.sortBtn, sort === id && styles.sortBtnActive)}
              >
                {label}
              </button>
            ))}
          </div>
          <button
            onClick={() => onCompilationsChange(!compilations)}
            className={cn(styles.compilationsBtn, compilations && styles.compilationsBtnActive)}
          >
            Compilations only
          </button>
        </div>
        {pagination && pagination.total_pages > 1 && (
          <Pagination page={page} totalPages={pagination.total_pages} onChange={onPageChange} />
        )}
      </div>

      {showLoader ? (
        <PageLoader />
      ) : isError ? (
        <ErrorState message="Failed to load albums" onRetry={refetch} />
      ) : albums.length === 0 ? (
        <EmptyState icon={Music} title="No albums yet" body="Scan your library to get started" />
      ) : (
        <div className={styles.grid}>
          {albums.map((album) => (
            <AlbumTile
              key={album.id}
              id={album.id}
              title={album.title}
              coverArtHash={album.cover_art_hash}
              artistName={album.primary_artist_name}
              releaseDate={album.release_date}
            />
          ))}
        </div>
      )}
    </div>
  )
}
