import { useEffect, useRef, useState } from 'react'
import { Moon, Search, LogOut, User, ChevronDown, X } from 'lucide-react'
import { useUIStore } from '@/stores/ui-store'
import { useAuthStore } from '@/features/auth/auth-store'
import { useLogout } from '@/features/auth/use-logout'
import { useNavigate, useLocation } from '@tanstack/react-router'
import { Toggle } from '@/components/ui/toggle'
import { SEARCH_DEBOUNCE_MS } from '@/lib/constants'
import { cn } from '@/lib/cn'
import styles from './topbar.module.css'

export function Topbar() {
  const { theme, toggleTheme } = useUIStore()
  const user = useAuthStore((s) => s.user)
  const { mutate: doLogout } = useLogout()
  const navigate = useNavigate()
  const location = useLocation()

  const isSearch = location.pathname === '/search'
  const searchQ = isSearch ? ((location.search as { q?: string }).q ?? '') : ''
  const [inputValue, setInputValue] = useState(searchQ)
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const searchEnteredRef = useRef(false)

  useEffect(() => {
    if (isSearch) setInputValue(searchQ)
    else searchEnteredRef.current = false
  }, [isSearch, searchQ])

  const handleSearch = (e: React.ChangeEvent<HTMLInputElement>) => {
    const val = e.target.value
    setInputValue(val)
    if (debounceRef.current) clearTimeout(debounceRef.current)
    debounceRef.current = setTimeout(() => {
      if (!searchEnteredRef.current) {
        searchEnteredRef.current = true
        navigate({ to: '/search', search: { q: val } })
      } else {
        navigate({ to: '/search', search: { q: val }, replace: true })
      }
    }, SEARCH_DEBOUNCE_MS)
  }

  const handleClear = () => {
    setInputValue('')
    if (debounceRef.current) clearTimeout(debounceRef.current)
    if (isSearch) {
      searchEnteredRef.current = false
      history.back()
    }
  }

  const [open, setOpen] = useState(false)
  const dropdownRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const handle = (e: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) setOpen(false)
    }
    if (open) document.addEventListener('mousedown', handle)
    return () => document.removeEventListener('mousedown', handle)
  }, [open])

  const initial = user?.username?.[0]?.toUpperCase() ?? '?'
  const isDark = theme === 'nocturne'

  return (
    <header className={styles.header}>
      <div className={styles.search}>
        <Search size={13} strokeWidth={1.5} />
        <input
          type="text"
          placeholder="Albums, artists, tracks…"
          value={inputValue}
          onChange={handleSearch}
          className={styles.searchInput}
        />
        {inputValue && (
          <button onClick={handleClear} aria-label="Clear search" className={styles.clearBtn}>
            <X size={12} strokeWidth={2} />
          </button>
        )}
      </div>

      <div className={styles.spacer} />

      <div ref={dropdownRef} className={styles.profileWrap}>
        <button
          onClick={() => setOpen((v) => !v)}
          aria-label="Account menu"
          aria-expanded={open}
          className={styles.avatarBtn}
        >
          <div className={styles.avatar}>{initial}</div>
          <ChevronDown
            size={13}
            strokeWidth={2}
            className={cn(styles.chevron, open && styles.chevronOpen)}
          />
        </button>

        {open && (
          <div className={styles.dropdown}>
            <div className={styles.dropdownProfile}>
              <div className={styles.avatarLarge}>{initial}</div>
              <div style={{ minWidth: 0 }}>
                <div className={styles.profileName}>{user?.username}</div>
                <div className={styles.profileRole}>
                  <User size={11} strokeWidth={1.5} />
                  {user?.isAdmin ? 'Administrator' : 'Member'}
                </div>
              </div>
            </div>

            <div className={cn(styles.dropdownItem, styles.toggleWrap)}>
              <span className={styles.dropdownItemInner}>
                <Moon size={14} strokeWidth={1.5} />
                Dark mode
              </span>
              <Toggle checked={isDark} onChange={toggleTheme} ariaLabel="Toggle dark mode" />
            </div>

            <button
              onClick={() => { setOpen(false); doLogout() }}
              className={styles.dropdownItem}
            >
              <LogOut size={14} strokeWidth={1.5} style={{ marginRight: 'var(--s-2)' }} />
              Sign out
            </button>
          </div>
        )}
      </div>
    </header>
  )
}
