import { X } from 'lucide-react'
import { useFocusTrap } from '@/hooks/use-focus-trap'
import { cn } from '@/lib/cn'
import styles from './modal.module.css'
import type { ModalProps } from './modal.types'

export function Modal({ open, onClose, title, size = 'sm', footer, children }: ModalProps) {
  const ref = useFocusTrap<HTMLDivElement>(open, onClose)
  if (!open) return null

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="modal-title"
      className={styles.overlay}
      onClick={(e) => { if (e.target === e.currentTarget) onClose() }}
    >
      <div ref={ref} className={cn(styles.dialog, styles[size])}>
        <div className={styles.header}>
          <h2 id="modal-title" className={styles.title}>{title}</h2>
          <button onClick={onClose} aria-label="Close" className={styles.closeBtn}>
            <X size={18} strokeWidth={1.5} />
          </button>
        </div>
        <div className={styles.body}>{children}</div>
        {footer && <div className={styles.footer}>{footer}</div>}
      </div>
    </div>
  )
}
