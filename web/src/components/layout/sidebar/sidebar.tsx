import { useState } from 'react'
import { Link, useNavigate, useLocation } from '@tanstack/react-router'
import { Home, BookOpen, Radio, Disc3, Mic2, ListMusic, Music2, Heart, Settings, ScanLine, Library } from 'lucide-react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { LogoWordmark } from '@/assets/LogoWordmark'
import { triggerScan } from '@/services/catalog'
import { useLibraries } from '@/hooks/use-libraries'
import { SettingsModal } from '@/features/library/settings-modal'
import { cn } from '@/lib/cn'
import styles from './sidebar.module.css'
import type { NavItemProps, NavTo } from './sidebar.types'

const TOP_NAV: readonly { to: NavTo; label: string; Icon: NavItemProps['Icon'] }[] = [
  { to: '/home',   label: 'Home',   Icon: Home },
  { to: '/browse', label: 'Browse', Icon: BookOpen },
  { to: '/radio',  label: 'Radio',  Icon: Radio },
]

const LIBRARY_NAV: readonly { to: NavTo; label: string; Icon: NavItemProps['Icon'] }[] = [
  { to: '/albums',    label: 'Albums',    Icon: Disc3 },
  { to: '/artists',   label: 'Artists',   Icon: Mic2 },
  { to: '/tracks',    label: 'Tracks',    Icon: Music2 },
  { to: '/playlists', label: 'Playlists', Icon: ListMusic },
  { to: '/loved',     label: 'Loved',     Icon: Heart },
]

function NavItem({ to, label, Icon }: NavItemProps) {
  const { pathname } = useLocation()
  const isActive = pathname.startsWith(to)

  return (
    <div className={styles.navItemWrap}>
      {isActive && <span className={styles.navItemIndicator} />}
      <Link
        to={to}
        className={cn(styles.navItem, isActive && styles.navItemActive)}
      >
        <Icon size={15} strokeWidth={1.5} style={{ flexShrink: 0 }} />
        {label}
      </Link>
    </div>
  )
}

export function Sidebar() {
  const [settingsOpen, setSettingsOpen] = useState(false)
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { libraries, activeLibrary, activeLibraryId, setActiveLibraryId } = useLibraries()

  const { mutate: scan, isPending: scanning } = useMutation({
    mutationFn: () => triggerScan(activeLibrary!.id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['libraries'] }),
  })

  return (
    <aside className={styles.sidebar}>
      <div className={styles.logo}>
        <div className={styles.logoInner}>
          <LogoWordmark height={34} />
        </div>
      </div>

      {libraries.length > 1 && (
        <div className={styles.switcher}>
          <select
            value={activeLibraryId ?? ''}
            onChange={(e) => { setActiveLibraryId(e.target.value); navigate({ to: '/albums' }) }}
            className={styles.switcherSelect}
          >
            {libraries.map((lib) => (
              <option key={lib.id} value={lib.id}>{lib.name}</option>
            ))}
          </select>
        </div>
      )}

      {libraries.length > 0 ? (
        <nav className={styles.nav}>
          <div className={styles.navGroup}>
            {TOP_NAV.map((item) => <NavItem key={item.to} {...item} />)}
          </div>

          <div className={styles.navGap} />

          <span className={styles.sectionLabel}>Your library</span>
          <div className={styles.navGroup}>
            {LIBRARY_NAV.map((item) => <NavItem key={item.to} {...item} />)}
          </div>

          <div className={styles.navGap} />

          <span className={styles.sectionLabel}>Library</span>
          <div className={styles.navGroup}>
            <NavItem to="/library" label="Manage" Icon={Library} />
          </div>
        </nav>
      ) : (
        <div className={styles.noLib}>
          <p className={styles.noLibText}>No libraries yet.</p>
          <Link to="/library" className={styles.noLibLink}>
            <Library size={13} strokeWidth={1.5} />
            Add a library
          </Link>
        </div>
      )}

      {activeLibrary && (
        <div className={styles.bottom}>
          <span className={styles.bottomName}>{activeLibrary.name}</span>
          <button
            onClick={() => scan()}
            disabled={scanning}
            aria-label="Scan library"
            title={scanning ? 'Scanning…' : 'Scan library'}
            className={styles.iconBtn}
          >
            <ScanLine size={12} strokeWidth={1.5} style={{ animation: scanning ? 'spin 1.4s linear infinite' : 'none' }} />
          </button>
          <button
            onClick={() => setSettingsOpen(true)}
            aria-label="Library settings"
            title="Library settings"
            className={cn(styles.iconBtn, styles.iconBtnBorderless)}
          >
            <Settings size={12} strokeWidth={1.5} />
          </button>
        </div>
      )}

      {settingsOpen && activeLibraryId && (
        <SettingsModal libraryId={activeLibraryId} onClose={() => setSettingsOpen(false)} />
      )}
    </aside>
  )
}
