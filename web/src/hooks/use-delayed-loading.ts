import { useEffect, useState } from 'react'

export function useDelayedLoading(isLoading: boolean, delayMs = 200): boolean {
  const [show, setShow] = useState(false)

  useEffect(() => {
    if (!isLoading) {
      setShow(false)
      return
    }
    const timer = setTimeout(() => setShow(true), delayMs)
    return () => clearTimeout(timer)
  }, [isLoading, delayMs])

  return show
}
