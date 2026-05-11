import { PriorityList } from './priority-list'
import styles from './settings-modal.module.css'
import type { LibrarySettings } from './settings-modal.types'

interface Props {
  settings: Partial<LibrarySettings>
  onChange: <K extends keyof LibrarySettings>(key: K, value: LibrarySettings[K]) => void
}

export function ArtworkTab({ settings, onChange }: Props) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--s-5)' }}>
      <div className={styles.artworkSection}>
        <p className={styles.sectionLabel}>Album art priority</p>
        <PriorityList items={settings.album_art_priority ?? []} onChange={(v) => onChange('album_art_priority', v)} />
      </div>
      <div className={styles.artworkSection}>
        <p className={styles.sectionLabel}>Artist image priority</p>
        <PriorityList items={settings.artist_image_priority ?? []} onChange={(v) => onChange('artist_image_priority', v)} />
      </div>
    </div>
  )
}
