import { cn } from '@/lib/cn'
import styles from './button.module.css'
import type { ButtonProps } from './button.types'

export function Button({
  variant = 'secondary',
  size = 'md',
  className,
  children,
  ...rest
}: ButtonProps) {
  return (
    <button
      type="button"
      {...rest}
      className={cn(styles.btn, styles[variant], styles[size], className)}
    >
      {children}
    </button>
  )
}
