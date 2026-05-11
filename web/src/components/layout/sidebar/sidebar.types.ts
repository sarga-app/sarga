import type { CSSProperties, FC } from 'react'

export type NavTo =
  | '/home' | '/browse' | '/radio'
  | '/albums' | '/artists' | '/tracks' | '/playlists' | '/loved'
  | '/library'

export interface NavItemProps {
  to: NavTo
  label: string
  Icon: FC<{ size: number; strokeWidth: number; style?: CSSProperties }>
}
