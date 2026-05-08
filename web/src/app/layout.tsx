import { Outlet } from '@tanstack/react-router'
import { Sidebar } from '@/components/sidebar'
import { Topbar } from '@/components/topbar'
import { PlayerBar } from '@/components/player-bar'

export function AppLayout() {
  return (
    <div style={{ display: 'flex', height: '100%', background: 'var(--bg)', overflow: 'hidden' }}>
      <Sidebar />
      <div style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
        <Topbar />
        <main
          style={{
            flex: 1,
            overflowY: 'auto',
            padding: 'var(--s-5)',
            paddingBottom: 'calc(76px + 16px + var(--s-5))',
          }}
        >
          <Outlet />
        </main>
      </div>
      <PlayerBar />
    </div>
  )
}
