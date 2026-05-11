import { Link } from '@tanstack/react-router'
import { Music } from 'lucide-react'
import { ImageFade } from '@/components/ui/image-fade'
import { artUrl } from '@/lib/constants'
import styles from './album-tile.module.css'
import type { AlbumTileProps } from './album-tile.types'

export function AlbumTile({ id, title, coverArtHash, artistName, releaseDate }: AlbumTileProps) {
  return (
    <Link to="/albums/$id" params={{ id }} className={styles.tile}>
      <div className={styles.artWrap}>
        {coverArtHash ? (
          <ImageFade
            src={artUrl(coverArtHash)}
            alt={title}
            placeholderStyle={{ width: '100%', aspectRatio: '1' }}
            style={{ width: '100%', aspectRatio: '1', objectFit: 'cover' }}
          />
        ) : (
          <div className={styles.placeholder}>
            <Music size={32} strokeWidth={1} color="var(--fg-4)" />
          </div>
        )}
      </div>
      <div className={styles.title}>{title}</div>
      {artistName && <div className={styles.artist}>{artistName}</div>}
      {releaseDate && <div className={styles.year}>{releaseDate.slice(0, 4)}</div>}
    </Link>
  )
}
