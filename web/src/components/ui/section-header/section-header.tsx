import styles from './section-header.module.css'
import type { SectionHeaderProps } from './section-header.types'

export function SectionHeader({ label, count, style }: SectionHeaderProps) {
  return (
    <div className={styles.header} style={style}>
      <span className={styles.label}>
        {count != null ? `${label} · ${count}` : label}
      </span>
      <div className={styles.rule} />
    </div>
  )
}
