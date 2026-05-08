import { Link } from '@tanstack/react-router'
import { Library } from 'lucide-react'
import { LogoWordmark } from '@/assets/LogoWordmark'

const NAV = [
  { to: '/library', label: 'Library', Icon: Library },
] as const

const baseNavStyle: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  gap: 'var(--s-3)',
  padding: '6px calc(var(--s-3) - 2px)',
  borderRadius: 'var(--r-input)',
  fontSize: 14,
  textDecoration: 'none',
  borderBottom: 'none',
  borderLeft: '2px solid transparent',
  color: 'var(--fg-2)',
  fontWeight: 400,
  transition: 'color var(--dur-micro) var(--ease)',
}

export function Sidebar() {
  return (
    <aside
      style={{
        width: 232,
        flexShrink: 0,
        background: 'var(--bg-1)',
        borderRight: '1px solid var(--border-strong)',
        display: 'flex',
        flexDirection: 'column',
        height: '100%',
        paddingBottom: 'calc(76px + 16px + 16px)',
      }}
    >
      <div style={{ padding: '18px var(--s-5) var(--s-4)' }}>
        <LogoWordmark height={24} />
      </div>

      <nav style={{ flex: 1, padding: '0 var(--s-3)' }}>
        {NAV.map(({ to, label, Icon }) => (
          <Link
            key={to}
            to={to}
            style={baseNavStyle}
            activeProps={{
              style: {
                ...baseNavStyle,
                color: 'var(--fg-1)',
                fontWeight: 600,
                borderLeft: '2px solid var(--accent)',
              },
            }}
          >
            <Icon size={16} strokeWidth={1.5} />
            {label}
          </Link>
        ))}
      </nav>
    </aside>
  )
}
