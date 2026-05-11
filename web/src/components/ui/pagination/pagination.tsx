import { ChevronLeft, ChevronRight } from 'lucide-react'
import { cn } from '@/lib/cn'
import styles from './pagination.module.css'
import type { PaginationProps } from './pagination.types'

function buildPages(page: number, totalPages: number): (number | '…')[] {
  if (totalPages <= 9) return Array.from({ length: totalPages }, (_, i) => i + 1)
  const left = Math.max(2, page - 2)
  const right = Math.min(totalPages - 1, page + 2)
  const pages: (number | '…')[] = [1]
  if (left > 2) pages.push('…')
  for (let i = left; i <= right; i++) pages.push(i)
  if (right < totalPages - 1) pages.push('…')
  pages.push(totalPages)
  return pages
}

export function Pagination({ page, totalPages, onChange }: PaginationProps) {
  if (totalPages <= 1) return null
  const pages = buildPages(page, totalPages)

  return (
    <div className={styles.root}>
      <button
        disabled={page === 1}
        onClick={() => onChange(page - 1)}
        aria-label="Previous page"
        className={styles.btn}
      >
        <ChevronLeft size={16} strokeWidth={2} />
      </button>

      {pages.map((p, i) =>
        p === '…' ? (
          <span key={`e${i}`} className={styles.ellipsis}>…</span>
        ) : (
          <button
            key={p}
            onClick={() => p !== page && onChange(p)}
            className={cn(styles.btn, p === page && styles.active)}
            aria-current={p === page ? 'page' : undefined}
          >
            {p}
          </button>
        )
      )}

      <button
        disabled={page === totalPages}
        onClick={() => onChange(page + 1)}
        aria-label="Next page"
        className={styles.btn}
      >
        <ChevronRight size={16} strokeWidth={2} />
      </button>
    </div>
  )
}
