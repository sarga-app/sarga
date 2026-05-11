import { cn } from '@/lib/cn'
import styles from './format-badge.module.css'
import type { FormatBadgeProps } from './format-badge.types'

export function FormatBadge({ format, highlight }: FormatBadgeProps) {
  if (!format) return null
  return (
    <span className={cn(styles.badge, highlight && styles.highlight)}>
      {format.toUpperCase()}
    </span>
  )
}
