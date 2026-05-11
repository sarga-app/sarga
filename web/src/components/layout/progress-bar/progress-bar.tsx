import { useIsFetching, useIsMutating } from '@tanstack/react-query'
import { useDelayedLoading } from '@/hooks/use-delayed-loading'
import styles from './progress-bar.module.css'

/** Indeterminate top bar that appears whenever any query/mutation is in flight. */
export function ProgressBar() {
  const fetching = useIsFetching()
  const mutating = useIsMutating()
  const active = fetching > 0 || mutating > 0
  // 150ms delay — fast queries don't flash the bar.
  const show = useDelayedLoading(active, 150)
  return show ? <div className={styles.bar} /> : null
}
