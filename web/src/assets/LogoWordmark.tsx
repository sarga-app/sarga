import WordmarkSvg from './logo-wordmark.svg?react'

interface Props {
  height?: number
  style?: React.CSSProperties
  className?: string
}

export function LogoWordmark({ height = 32, style, className }: Props) {
  return (
    <WordmarkSvg
      height={height}
      width={(192 / 80) * height}
      style={{ color: 'var(--fg-1)', ...style }}
      className={className}
    />
  )
}
