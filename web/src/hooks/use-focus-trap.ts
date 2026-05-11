import { useEffect, useRef } from 'react'

/**
 * Trap focus inside a container while open. Restores focus to the
 * previously focused element on unmount. Closes on Escape.
 */
export function useFocusTrap<T extends HTMLElement>(open: boolean, onClose: () => void) {
  const ref = useRef<T>(null)

  useEffect(() => {
    if (!open) return
    const root = ref.current
    if (!root) return

    const previouslyFocused = document.activeElement as HTMLElement | null

    // Focus the first focusable element in the dialog
    const focusables = root.querySelectorAll<HTMLElement>(
      'a[href], button:not([disabled]), textarea, input, select, [tabindex]:not([tabindex="-1"])'
    )
    focusables[0]?.focus()

    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault()
        onClose()
        return
      }
      if (e.key !== 'Tab') return
      const items = Array.from(
        root.querySelectorAll<HTMLElement>(
          'a[href], button:not([disabled]), textarea, input, select, [tabindex]:not([tabindex="-1"])'
        ),
      ).filter((el) => !el.hasAttribute('aria-hidden'))
      if (items.length === 0) return
      const first = items[0]
      const last = items[items.length - 1]
      const active = document.activeElement as HTMLElement
      if (e.shiftKey && active === first) {
        last.focus()
        e.preventDefault()
      } else if (!e.shiftKey && active === last) {
        first.focus()
        e.preventDefault()
      }
    }

    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('keydown', onKey)
      previouslyFocused?.focus?.()
    }
  }, [open, onClose])

  return ref
}
