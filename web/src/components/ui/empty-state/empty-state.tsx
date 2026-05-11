import styles from './empty-state.module.css'
import type { EmptyStateProps, ErrorStateProps, ComingSoonProps } from './empty-state.types'

export function EmptyState({ icon: Icon, title, body }: EmptyStateProps) {
  return (
    <div className={styles.box}>
      <Icon size={48} strokeWidth={1} color="var(--fg-4)" />
      <div>
        <p className={styles.title}>{title}</p>
        {body && <p className={styles.body}>{body}</p>}
      </div>
    </div>
  )
}

export function ErrorState({ message, onRetry }: ErrorStateProps) {
  return (
    <div className={styles.box}>
      <p style={{ color: 'var(--fg-3)', margin: 0 }}>{message}</p>
      {onRetry && (
        <button onClick={onRetry} className={styles.retryBtn}>Retry</button>
      )}
    </div>
  )
}

export function ComingSoon({ icon: Icon, title, body }: ComingSoonProps) {
  return (
    <div className={styles.box}>
      <Icon size={48} strokeWidth={1} color="var(--fg-4)" />
      <div>
        <p className={styles.title}>{title}</p>
        {body && <p className={styles.body}>{body}</p>}
      </div>
    </div>
  )
}
