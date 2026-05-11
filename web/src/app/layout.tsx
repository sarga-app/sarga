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
  // useLibraries owns the libraries query + auto-pick effect
  useLibraries()

  const location = useLocation()
  const prevPathRef = useRef(location.pathname)
  const prevKeyRef = useRef(scrollKey(location))

  // Scroll restoration. Runs synchronously *before* paint to avoid flash.
  useLayoutEffect(() => {
    const main = document.getElementById(SCROLL_ID)
    if (!main) return

    const samePath = prevPathRef.current === location.pathname
    const currKey = scrollKey(location)

    sessionStorage.setItem(prevKeyRef.current, String(main.scrollTop))

    // Same-path nav (pagination/sort) → keep scroll. New path → restore or top.
    if (!samePath) {
      const saved = sessionStorage.getItem(currKey)
      const prev = main.style.scrollBehavior
      main.style.scrollBehavior = 'auto'
      main.scrollTop = saved !== null ? parseInt(saved, 10) : 0
      requestAnimationFrame(() => { main.style.scrollBehavior = prev })
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
          <div key={location.pathname} className={styles.page}>
            <Suspense fallback={<PageLoader />}>
              <Outlet />
            </Suspense>
          </div>
        </main>
      </div>
      <PlayerBar />
    </div>
  )
}
