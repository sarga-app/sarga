import { Link } from '@tanstack/react-router'
import { Music } from 'lucide-react'
import { artUrl } from '@/lib/constants'
import styles from './album-tile.module.css'
import type { AlbumTileProps } from './album-tile.types'

export function AlbumTile({ id, title, coverArtHash, artistName, releaseDate }: AlbumTileProps) {
  return (
    <Link to="/albums/$id" params={{ id }} className={styles.tile}>
      <div className={styles.art}>
        {coverArtHash ? (
          <img src={artUrl(coverArtHash, 480)} alt="" loading="lazy" decoding="async" />
        ) : (
          <Music size={32} strokeWidth={1} color="var(--fg-4)" />
        )}
      </div>
      <div className={styles.title}>{title}</div>
      {artistName && <div className={styles.artist}>{artistName}</div>}
      {releaseDate && <div className={styles.year}>{releaseDate.slice(0, 4)}</div>}
    </Link>
  )
}
