import { useUIStore } from '@/stores/ui-store'

export function PlayerBar() {
  const theme = useUIStore((s) => s.theme)

  return (
    <div
      className={theme === 'nocturne' ? 'surface-vellum' : 'surface-lacquer'}
      style={{
        position: 'fixed',
        left: 16,
        right: 16,
        bottom: 16,
        height: 76,
        borderRadius: 10,
        display: 'flex',
        alignItems: 'center',
        padding: '0 var(--s-5)',
        zIndex: 100,
        boxShadow: '0 8px 32px rgba(0,0,0,0.20)',
      }}
    >
      <span
        style={{
          fontFamily: 'var(--font-mono)',
          fontSize: 12,
          color: 'var(--fg-3)',
        }}
      >
        Nothing playing
      </span>
    </div>
  )
}
