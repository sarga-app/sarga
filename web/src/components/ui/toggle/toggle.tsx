import { cn } from '@/lib/cn'
import styles from './toggle.module.css'
import type { ToggleProps } from './toggle.types'

export function Toggle({ checked, onChange, ariaLabel }: ToggleProps) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={ariaLabel}
      onClick={() => onChange(!checked)}
      className={cn(styles.track, checked && styles.trackOn)}
    >
      <span className={cn(styles.thumb, checked && styles.thumbOn)} />
    </button>
  )
}
