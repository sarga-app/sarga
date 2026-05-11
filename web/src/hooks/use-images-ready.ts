import { useEffect, useState } from 'react'

/** Hard cap so a single slow image doesn't trap the spinner forever. */
const MAX_WAIT_MS = 2500

/**
 * Pre-loads the given image URLs and resolves to `true` when all are decoded
 * (or after MAX_WAIT_MS). Use this at the page level so the page reveals
 * itself fully painted instead of with progressively-filling placeholders.
 *
 *   const ready = useImagesReady(albums.map(a => artUrl(a.cover_art_hash)))
 *   if (isLoading || (albums.length > 0 && !ready)) return <PageLoader />
 *
 * - `null` / `undefined` URLs are skipped.
 * - When the URL set is empty, ready is true immediately.
 * - The dependency is a stable join key so React doesn't re-fire on every render.
 */
export function useImagesReady(srcs: (string | null | undefined)[]): boolean {
  const valid = srcs.filter((s): s is string => Boolean(s))
  const key = valid.join('|')
  const [readyKey, setReadyKey] = useState<string | null>(valid.length === 0 ? key : null)

  useEffect(() => {
    if (valid.length === 0) { setReadyKey(key); return }

    let cancelled = false
    const finish = () => { if (!cancelled) setReadyKey(key) }

    const timer = setTimeout(finish, MAX_WAIT_MS)

    Promise.all(
      valid.map(
        (src) =>
          new Promise<void>((resolve) => {
            const img = new Image()
            img.onload = img.onerror = () => resolve()
            img.src = src
          }),
      ),
    ).then(() => {
      clearTimeout(timer)
      finish()
    })

    return () => {
      cancelled = true
      clearTimeout(timer)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key])

  return readyKey === key
}
