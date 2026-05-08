import { Search } from 'lucide-react'

export function Topbar() {
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
    </header>
  )
}
