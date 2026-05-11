import MarkSvg from './logo-mark.svg?react'

interface Props {
  size?: number
  style?: React.CSSProperties
  className?: string
}

export function LogoMark({ size = 32, style, className }: Props) {
  return <MarkSvg width={size} height={size} style={style} className={className} />
}
