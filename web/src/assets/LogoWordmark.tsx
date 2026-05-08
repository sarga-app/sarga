interface Props {
  height?: number
  style?: React.CSSProperties
  className?: string
}

export function LogoWordmark({ height = 28, style, className }: Props) {
  const width = (320 / 80) * height

  return (
    <svg
      viewBox="0 0 320 80"
      width={width}
      height={height}
      style={style}
      className={className}
      aria-label="Sarga"
    >
      <text
        x="0"
        y="60"
        style={{
          fontFamily: 'var(--font-editorial)',
          fontSize: 72,
          fontWeight: 500,
          fill: 'var(--fg-1)',
          letterSpacing: '-0.015em',
        }}
      >
        sarga
      </text>
      <rect x="178" y="14" width="3" height="48" style={{ fill: 'var(--accent)' }} />
      <rect x="186" y="14" width="3" height="48" style={{ fill: 'var(--accent)' }} />
    </svg>
  )
}
