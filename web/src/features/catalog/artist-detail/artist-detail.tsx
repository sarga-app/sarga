import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getArtist } from '@/services/catalog'
import { SectionHeader } from '@/components/ui/section-header'
import { AlbumTile } from '@/components/ui/album-tile'
import { PageLoader } from '@/components/ui/page-loader'
import { cn } from '@/lib/cn'
import styles from './artist-detail.module.css'
import type { ArtistDetailProps } from './artist-detail.types'

export function ArtistDetail({ id, libraryId }: ArtistDetailProps) {
  const [bioExpanded, setBioExpanded] = useState(false)

  const { data, isLoading, isError } = useQuery({
    queryKey: ['artist', id, libraryId],
    queryFn: () => getArtist(id, libraryId),
  })

  if (isLoading) return <PageLoader />

  if (isError || !data) {
    return (
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: 'var(--s-4)', padding: 'var(--s-10) 0' }}>
        <p style={{ color: 'var(--fg-2)', margin: 0 }}>
          {isError ? 'Failed to load artist' : 'Artist not found'}
        </p>
      </div>
    )
  }

  const careerSpan = data.begin_year
    ? `${data.begin_year}–${data.end_year ?? 'present'}`
    : null

  const sortedReleases = [...data.releases].sort((a, b) => {
    const ay = a.release_date?.slice(0, 4) ?? '0'
    const by = b.release_date?.slice(0, 4) ?? '0'
    return by.localeCompare(ay)
  })

  return (
    <div>
      <div className={styles.hero}>
        <div className={styles.heroContent}>
          <div className={styles.heroMeta}>
            <h1 className={styles.heroName}>{data.name}</h1>
            <div className={styles.heroDetails}>
              {data.country && <span className={styles.heroDetail}>{data.country}</span>}
              {careerSpan && <span className={styles.heroDetail}>{careerSpan}</span>}
            </div>
          </div>
        </div>
      </div>

      {data.bio && (
        <div className={styles.bio}>
          <p className={cn(styles.bioText, !bioExpanded && styles.bioCollapsed)}>{data.bio}</p>
          <button onClick={() => setBioExpanded((v) => !v)} className={styles.bioBtn}>
            {bioExpanded ? 'Show less' : 'Read more'}
          </button>
        </div>
      )}

      <div>
        <SectionHeader label="Discography" style={{ marginBottom: 'var(--s-4)' }} />
        {sortedReleases.length === 0 ? (
          <p style={{ color: 'var(--fg-3)', fontSize: 14 }}>No releases found</p>
        ) : (
          <div className={styles.discoGrid}>
            {sortedReleases.map((album) => (
              <AlbumTile
                key={album.id}
                id={album.id}
                title={album.title}
                coverArtHash={album.cover_art_hash}
                releaseDate={album.release_date}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
