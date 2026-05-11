import { ChevronUp, ChevronDown } from 'lucide-react'
import styles from './settings-modal.module.css'
import type { PriorityListProps } from './settings-modal.types'

export function PriorityList({ items, onChange }: PriorityListProps) {
  const move = (idx: number, dir: -1 | 1) => {
    const next = [...items]
    const swap = idx + dir
    if (swap < 0 || swap >= next.length) return
    ;[next[idx], next[swap]] = [next[swap], next[idx]]
    onChange(next)
  }

  return (
    <div className={styles.priorityList}>
      {items.map((item, i) => (
        <div key={item} className={styles.priorityItem}>
          <span className={styles.priorityItemName}>{item}</span>
          <button
            type="button"
            onClick={() => move(i, -1)}
            disabled={i === 0}
            aria-label={`Move ${item} up`}
            className={styles.priorityBtn}
          >
            <ChevronUp size={14} />
          </button>
          <button
            type="button"
            onClick={() => move(i, 1)}
            disabled={i === items.length - 1}
            aria-label={`Move ${item} down`}
            className={styles.priorityBtn}
          >
            <ChevronDown size={14} />
          </button>
        </div>
      ))}
    </div>
  )
}
