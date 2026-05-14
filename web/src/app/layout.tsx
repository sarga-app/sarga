import { Outlet, useLocation } from '@tanstack/react-router'
import { Suspense, useLayoutEffect, useRef } from 'react'
import { Sidebar } from '@/components/layout/sidebar'
import { Topbar } from '@/components/layout/topbar'
import { PlayerBar } from '@/components/layout/player-bar'
import { ProgressBar } from '@/components/layout/progress-bar'
import { PageLoader } from '@/components/ui/page-loader'
import { useLibraries } from '@/hooks/use-libraries'
import styles from './layout.module.css'

const SCROLL_ID = 'main-scroll'

function scrollKey(loc: { pathname: string; searchStr: string }) {
  return `scroll:${loc.pathname}${loc.searchStr}`
}

export function AppLayout() {
  useLibraries()

  const location = useLocation()
  const prevPathRef = useRef(location.pathname)
  const prevKeyRef = useRef(scrollKey(location))

  useLayoutEffect(() => {
    const main = document.getElementById(SCROLL_ID)
    if (!main) return

    const samePath = prevPathRef.current === location.pathname
    const currKey = scrollKey(location)

    sessionStorage.setItem(prevKeyRef.current, String(main.scrollTop))
    if (!samePath) {
      const saved = sessionStorage.getItem(currKey)
      main.scrollTop = saved !== null ? parseInt(saved, 10) : 0
    }

    prevPathRef.current = location.pathname
    prevKeyRef.current = currKey
  }, [location])

  return (
    <div className={styles.shell}>
      <ProgressBar />
      <Sidebar />
      <div className={styles.column}>
        <Topbar />
        <main id={SCROLL_ID} className={styles.main}>
          <Suspense fallback={<PageLoader />}>
            <Outlet />
          </Suspense>
        </main>
      </div>
      <PlayerBar />
    </div>
  )
}
