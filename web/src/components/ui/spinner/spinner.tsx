import styles from './spinner.module.css'
import type { SpinnerProps } from './spinner.types'

export function Spinner({ size = 36 }: SpinnerProps) {
  // Thicker stroke ratio so the arc is clearly visible at any size.
  const stroke = Math.max(3, Math.round(size / 9))
  return (
    <span
      role="status"
      aria-label="Loading"
      className={styles.spinner}
      style={{ width: size, height: size, borderWidth: stroke }}
    />
  )
}
