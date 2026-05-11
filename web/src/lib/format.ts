/**
 * Format a duration in milliseconds as M:SS.
 * Safety net: clamps malformed values (some legacy data was stored as ms × 1000
 * because of a Vorbis DURATION-in-ms misinterpretation in the scanner).
 */
const MAX_REASONABLE_MS = 24 * 60 * 60 * 1000

export function formatDuration(ms: number | null | undefined): string {
  if (!ms || ms <= 0) return '—'
  let v = ms
  while (v > MAX_REASONABLE_MS) v = Math.round(v / 1000)
  const s = Math.round(v / 1000)
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`
}
