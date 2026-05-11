import type { ReactNode } from 'react'

export interface ModalProps {
  open: boolean
  onClose: () => void
  title: string
  /** sm = 460px (forms), md = 520px (settings/tabs) */
  size?: 'sm' | 'md'
  footer?: ReactNode
  children: ReactNode
}
