import { useState, useEffect } from 'react'
import { Spinner } from '@/components/ui/spinner'
import { SKELETON_DELAY_MS } from '@/lib/constants'
import styles from './page-loader.module.css'

export function PageLoader() {
  const [visible, setVisible] = useState(false)

  useEffect(() => {
    const t = setTimeout(() => setVisible(true), SKELETON_DELAY_MS)
    return () => clearTimeout(t)
  }, [])

  if (!visible) return null

  return (
    <div className={styles.center}>
      <Spinner size={36} />
    </div>
  )
}
