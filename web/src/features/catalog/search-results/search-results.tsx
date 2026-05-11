import { Link } from '@tanstack/react-router'
import { useQuery, keepPreviousData } from '@tanstack/react-query'
import { Search, Music } from 'lucide-react'
import { search } from '@/services/catalog'
import { ImageFade } from '@/components/ui/image-fade'
import { FormatBadge } from '@/components/ui/format-badge'
import { SectionHeader } from '@/components/ui/section-header'
import { EmptyState } from '@/components/ui/empty-state'
import { PageLoader } from '@/components/ui/page-loader'
import { artUrl } from '@/lib/constants'
import styles from './search-results.module.css'
import type { Album, Artist, Track, SearchResultsProps } from './search-results.types'

function AlbumRow({ album }: { album: Album }) {
  return (
    <Link to="/albums/$id" params={{ id: album.id }} className={styles.albumRow}>
      <div className={styles.albumArt}>
        {album.cover_art_hash ? (
          <ImageFade
            src={artUrl(album.cover_art_hash)}
            alt={album.title}
            placeholderStyle={{ width: '100%', height: '100%' }}
            style={{ width: '100%', height: '100%', objectFit: 'cover' }}
          />
        ) : (
          <Music size={16} strokeWidth={1} color="var(--fg-4)" />
        )}
      </div>
      <div className={styles.albumMeta}>
        <div className={styles.albumTitle}>{album.title}</div>
        {album.primary_artist_name && (
          <div className={styles.albumArtist}>{album.primary_artist_name}</div>
        )}
      </div>
      {album.release_date && (
        <span className={styles.albumYear}>{album.release_date.slice(0, 4)}</span>
      )}
    </Link>
  )
}

function ArtistRow({ artist }: { artist: Artist }) {
  return (
    <Link to="/artists/$id" params={{ id: artist.id }} className={styles.artistRow}>
      <span className={styles.artistName}>{artist.name}</span>
      {artist.release_count != null && (
        <span className={styles.artistCount}>{artist.release_count} releases</span>
      )}
    </Link>
  )
}

function TrackRow({ track }: { track: Track }) {
  return (
    <div className={styles.trackRow}>
      <div className={styles.trackMeta}>
        <div className={styles.trackTitle}>{track.title}</div>
        {track.primary_artist_name && (
          <div className={styles.trackSub}>
            {track.primary_artist_name}
            {track.release_title ? ` · ${track.release_title}` : ''}
          </div>
        )}
      </div>
      <FormatBadge format={track.format} />
    </div>
  )
}

export function SearchResults({ libraryId, query }: SearchResultsProps) {
  const { data, isLoading } = useQuery({
    queryKey: ['search', libraryId, query],
    queryFn: ({ signal }) => search(query, libraryId, 1, 20, signal),
    enabled: query.length > 0,
    placeholderData: keepPreviousData,
  })

  const albums = data?.albums.data ?? []
  const artists = data?.artists.data ?? []
  const tracks = data?.tracks.data ?? []

  if (!query) {
    return <EmptyState icon={Search} title="Search your library" body="Albums, artists, tracks" />
  }

  if (isLoading) return <PageLoader />

  const totalAlbums = data?.albums.total ?? 0
  const totalArtists = data?.artists.total ?? 0
  const totalTracks = data?.tracks.total ?? 0
  const hasResults = totalAlbums + totalArtists + totalTracks > 0

  if (!hasResults) {
    return <EmptyState icon={Search} title={`No results for "${query}"`} />
  }

  return (
    <div>
      {albums.length > 0 && (
        <div>
          <SectionHeader label="Albums" count={totalAlbums} style={{ marginBottom: 'var(--s-3)', marginTop: 'var(--s-6)' }} />
          {albums.map((a) => <AlbumRow key={a.id} album={a} />)}
        </div>
      )}
      {artists.length > 0 && (
        <div>
          <SectionHeader label="Artists" count={totalArtists} style={{ marginBottom: 'var(--s-3)', marginTop: 'var(--s-6)' }} />
          {artists.map((a) => <ArtistRow key={a.id} artist={a} />)}
        </div>
      )}
      {tracks.length > 0 && (
        <div>
          <SectionHeader label="Tracks" count={totalTracks} style={{ marginBottom: 'var(--s-3)', marginTop: 'var(--s-6)' }} />
          {tracks.map((t) => <TrackRow key={t.id} track={t} />)}
        </div>
      )}
    </div>
  )
}
