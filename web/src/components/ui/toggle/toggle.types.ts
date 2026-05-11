export interface ToggleProps {
  checked: boolean
  onChange: (v: boolean) => void
  /** Optional aria-label when there's no visible label */
  ariaLabel?: string
}
