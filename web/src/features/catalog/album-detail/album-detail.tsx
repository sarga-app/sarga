import { Link } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { Music } from 'lucide-react'
import { getAlbum } from '@/services/catalog'
import { ImageFade } from '@/components/ui/image-fade'
import { SectionHeader } from '@/components/ui/section-header'
import { TrackRow } from '@/components/ui/track-row'
import { PageLoader } from '@/components/ui/page-loader'
import { artUrl } from '@/lib/constants'
import styles from './album-detail.module.css'
import type { AlbumDetailProps } from './album-detail.types'

export function AlbumDetail({ id, libraryId }: AlbumDetailProps) {
  const { data, isLoading, isError } = useQuery({
    queryKey: ['album', id, libraryId],
    queryFn: () => getAlbum(id, libraryId),
  })

  if (isLoading) return <PageLoader />

  if (isError || !data) {
    return (
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: 'var(--s-4)', padding: 'var(--s-10) 0', textAlign: 'center' }}>
        <p style={{ color: 'var(--fg-2)', margin: 0 }}>
          {isError ? 'Failed to load album' : 'Album not found'}
        </p>
      </div>
    )
  }

  const year = data.release_date?.slice(0, 4)

  const tracksByDisc = data.tracks.reduce<Record<number, typeof data.tracks>>((acc, t) => {
    const d = t.disc_number ?? 1
    acc[d] = acc[d] ?? []
    acc[d].push(t)
    return acc
  }, {})

  const discNumbers = Object.keys(tracksByDisc).map(Number).sort((a, b) => a - b)
  const multiDisc = (data.total_discs ?? 1) > 1 || discNumbers.length > 1

  return (
    <div>
      <div className={styles.hero}>
        <div className={styles.artBox}>
          {data.cover_art_hash ? (
            <ImageFade
              src={artUrl(data.cover_art_hash, 640)}
              alt={data.title}
              placeholderStyle={{ width: '100%', height: '100%' }}
              style={{ width: '100%', height: '100%', objectFit: 'cover' }}
            />
          ) : (
            <Music size={48} strokeWidth={1} color="var(--fg-4)" />
          )}
        </div>

        <div className={styles.meta}>
          {data.edition_notes && (
            <span className={styles.editionBadge}>{data.edition_notes}</span>
          )}
          <h1 className={styles.title}>{data.title}</h1>

          {data.primary_artist_id ? (
            <Link to="/artists/$id" params={{ id: data.primary_artist_id }} className={styles.artistLink}>
              {data.primary_artist_name}
            </Link>
          ) : (
            data.primary_artist_name && (
              <p className={styles.artistText}>{data.primary_artist_name}</p>
            )
          )}

          <div className={styles.metaRow}>
            {year && <span className={styles.metaMono}>{year}</span>}
            {data.country && <span className={styles.metaMono}>{data.country}</span>}
          </div>

          {(data.label_name || data.catalog_number) && (
            <p className={styles.labelLine}>
              {[data.label_name, data.catalog_number].filter(Boolean).join(' · ')}
            </p>
          )}
          {data.release_type && (
            <span className={styles.releaseBadge}>{data.release_type}</span>
          )}
        </div>
      </div>

      <div>
        {discNumbers.map((disc) => (
          <div key={disc}>
            {multiDisc && (
              <SectionHeader
                label={`Disc ${disc}`}
                style={{ marginBottom: 'var(--s-3)', marginTop: disc > discNumbers[0] ? 'var(--s-5)' : 0 }}
              />
            )}
            <div>
              {(tracksByDisc[disc] ?? []).map((track) => (
                <TrackRow
                  key={track.track_id}
                  position={track.position}
                  title={track.title}
                  format={track.format}
                  durationMs={track.duration_ms}
                  missing={!track.track_file_id}
                />
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
