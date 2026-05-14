import { Link } from '@tanstack/react-router'
import { useQuery, keepPreviousData } from '@tanstack/react-query'
import { Mic2 } from 'lucide-react'
import { listArtists } from '@/services/catalog'
import { Pagination } from '@/components/ui/pagination'
import { EmptyState, ErrorState } from '@/components/ui/empty-state'
import { PageLoader } from '@/components/ui/page-loader'
import { ARTISTS_PAGE_SIZE } from '@/lib/constants'
import styles from './artists-list.module.css'
import type { ArtistsListProps } from './artists-list.types'

const ALPHA = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ#'.split('')

function prefersReducedMotion() {
  return typeof window !== 'undefined' &&
    window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
}

export function ArtistsList({ libraryId, page, onPageChange }: ArtistsListProps) {
  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ['artists', libraryId, page],
    queryFn: () => listArtists(libraryId, page, ARTISTS_PAGE_SIZE),
    placeholderData: keepPreviousData,
  })

  const artists = data?.data ?? []
  const pagination = data?.pagination

  const grouped = artists.reduce<Record<string, typeof artists>>((acc, artist) => {
    const letter = (artist.sort_name ?? artist.name)[0]?.toUpperCase() ?? '#'
    const key = /[A-Z]/.test(letter) ? letter : '#'
    acc[key] = acc[key] ?? []
    acc[key].push(artist)
    return acc
  }, {})

  const letters = Object.keys(grouped).sort((a, b) => {
    if (a === '#') return 1
    if (b === '#') return -1
    return a.localeCompare(b)
  })

  const scrollTo = (letter: string) => {
    const el = document.querySelector<HTMLDivElement>(`[data-section-letter="${CSS.escape(letter)}"]`)
    el?.scrollIntoView({ behavior: prefersReducedMotion() ? 'auto' : 'smooth', block: 'start' })
  }

  if (isLoading) return <PageLoader />

  if (isError) return <ErrorState message="Failed to load artists" onRetry={refetch} />

  if (artists.length === 0) {
    return <EmptyState icon={Mic2} title="No artists yet" body="Scan your library to get started" />
  }

  return (
    <div className={styles.wrap}>
      <div className={styles.list}>
        {pagination && pagination.total_pages > 1 && (
          <Pagination page={page} totalPages={pagination.total_pages} onChange={onPageChange} />
        )}

        {letters.map((letter) => (
          <div key={letter} data-section-letter={letter}>
            <div className={styles.letterHeader}>
              <span className={styles.letterLabel}>{letter}</span>
              <div className={styles.letterRule} />
            </div>

            {(grouped[letter] ?? []).map((artist) => (
              <Link
                key={artist.id}
                to="/artists/$id"
                params={{ id: artist.id }}
                className={styles.artistRow}
              >
                <span className={styles.artistName}>{artist.name}</span>
                <div className={styles.artistMeta}>
                  {artist.country && <span className={styles.artistCountry}>{artist.country}</span>}
                  {artist.release_count != null && (
                    <span className={styles.artistCount}>{artist.release_count}</span>
                  )}
                </div>
              </Link>
            ))}
          </div>
        ))}
      </div>

      <div className={styles.jumpNav}>
        {ALPHA.filter((l) => letters.includes(l)).map((letter) => (
          <button key={letter} onClick={() => scrollTo(letter)} className={styles.jumpBtn}>
            {letter}
          </button>
        ))}
      </div>
    </div>
  )
}
