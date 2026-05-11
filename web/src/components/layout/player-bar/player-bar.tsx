import { useUIStore } from '@/stores/ui-store'
import { cn } from '@/lib/cn'
import styles from './player-bar.module.css'

export function PlayerBar() {
  const theme = useUIStore((s) => s.theme)
  return (
    <div className={cn(styles.bar, theme === 'nocturne' ? 'surface-vellum' : 'surface-lacquer')}>
      <span className={styles.placeholder}>Nothing playing</span>
    </div>
  )
}
