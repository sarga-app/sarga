interface Props {
  size?: number
  style?: React.CSSProperties
  className?: string
}

export function LogoMark({ size = 32, style, className }: Props) {
  return (
    <svg
      viewBox="0 0 64 64"
      width={size}
      height={size}
      style={style}
      className={className}
      aria-label="Sarga"
    >
      <rect width="64" height="64" rx="8" fill="var(--ink-1)" />
      <rect x="22" y="14" width="4" height="36" fill="var(--paper-0)" />
      <rect x="32" y="14" width="4" height="36" fill="var(--paper-0)" />
      <rect x="42" y="14" width="4" height="36" fill="var(--accent-vinyl)" />
    </svg>
  )
}
