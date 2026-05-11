import { FormatBadge } from '@/components/ui/format-badge'
import { formatDuration } from '@/lib/format'
import { cn } from '@/lib/cn'
import styles from './track-row.module.css'
import type { TrackRowProps } from './track-row.types'

export function TrackRow({ position, title, format, durationMs, missing }: TrackRowProps) {
  return (
    <div className={cn(styles.row, missing && styles.missing)}>
      <span className={styles.num}>{String(position).padStart(2, '0')}</span>
      <span className={styles.title}>
        {title}
        {missing && <span className={styles.missingLabel}>(missing)</span>}
      </span>
      <FormatBadge format={format} />
      <span className={styles.duration}>{formatDuration(durationMs)}</span>
    </div>
  )
}
