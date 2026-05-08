import { LogOut, Moon, Search, Sun } from 'lucide-react'
import { useUIStore } from '@/stores/ui-store'
import { useAuthStore } from '@/features/auth/auth-store'
import { useLogout } from '@/features/auth/use-logout'

const iconBtnStyle: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  width: 32,
  height: 32,
  flexShrink: 0,
  borderRadius: 'var(--r-input)',
  border: '1px solid var(--border)',
  background: 'transparent',
  color: 'var(--fg-3)',
  cursor: 'pointer',
  transition: 'color var(--dur-micro) var(--ease)',
}

export function Topbar() {
  const { theme, toggleTheme } = useUIStore()
  const user = useAuthStore((s) => s.user)
  const { mutate: doLogout, isPending } = useLogout()

  const initial = user?.username?.[0]?.toUpperCase() ?? '?'

  return (
    <header
      style={{
        height: 56,
        margin: '12px 12px 0',
        flexShrink: 0,
        background: 'var(--bg-1)',
        border: '1px solid var(--border)',
        borderRadius: 'var(--r-sheet)',
        display: 'flex',
        alignItems: 'center',
        padding: '0 var(--s-4)',
        gap: 'var(--s-3)',
      }}
    >
      {/* Search */}
      <div
        style={{
          flex: 1,
          maxWidth: 480,
          display: 'flex',
          alignItems: 'center',
          gap: 'var(--s-2)',
          background: 'var(--bg)',
          border: '1px solid var(--border)',
          borderRadius: 'var(--r-input)',
          padding: '0 var(--s-3)',
          height: 32,
          color: 'var(--fg-3)',
        }}
      >
        <Search size={14} strokeWidth={1.5} />
        <input
          type="search"
          placeholder="Search"
          style={{
            flex: 1,
            border: 'none',
            background: 'transparent',
            color: 'var(--fg-1)',
            fontFamily: 'var(--font-ui)',
            fontSize: 14,
            outline: 'none',
          }}
        />
      </div>

      {/* Theme toggle */}
      <button
        onClick={toggleTheme}
        aria-label={theme === 'nocturne' ? 'Switch to light mode' : 'Switch to dark mode'}
        style={iconBtnStyle}
      >
        {theme === 'nocturne'
          ? <Sun size={15} strokeWidth={1.5} />
          : <Moon size={15} strokeWidth={1.5} />}
      </button>

      {/* User avatar */}
      <div
        title={user?.username}
        style={{
          width: 28,
          height: 28,
          borderRadius: '50%',
          background: 'var(--bg-3)',
          color: 'var(--fg-2)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          fontFamily: 'var(--font-mono)',
          fontSize: 11,
          fontWeight: 500,
          flexShrink: 0,
          userSelect: 'none',
        }}
      >
        {initial}
      </div>

      {/* Logout */}
      <button
        onClick={() => doLogout()}
        disabled={isPending}
        aria-label="Sign out"
        title="Sign out"
        style={{ ...iconBtnStyle, border: 'none' }}
      >
        <LogOut size={15} strokeWidth={1.5} />
      </button>
    </header>
  )
}
