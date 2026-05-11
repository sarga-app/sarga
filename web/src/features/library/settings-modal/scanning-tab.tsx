import { Toggle } from '@/components/ui/toggle'
import { cn } from '@/lib/cn'
import styles from './settings-modal.module.css'
import type { LibrarySettings } from './settings-modal.types'

interface Props {
  settings: Partial<LibrarySettings>
  onChange: <K extends keyof LibrarySettings>(key: K, value: LibrarySettings[K]) => void
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className={styles.toggle}>
      <span className={styles.toggleLabel}>{label}</span>
      {children}
    </label>
  )
}

export function ScanningTab({ settings, onChange }: Props) {
  return (
    <div>
      <Row label="Scan on create">
        <Toggle checked={settings.scan_on_create ?? true} onChange={(v) => onChange('scan_on_create', v)} ariaLabel="Scan on create" />
      </Row>
      <Row label="Watch for changes">
        <Toggle checked={settings.watch_for_changes ?? true} onChange={(v) => onChange('watch_for_changes', v)} ariaLabel="Watch for changes" />
      </Row>
      <Row label="Follow symlinks">
        <Toggle checked={settings.follow_symlinks ?? true} onChange={(v) => onChange('follow_symlinks', v)} ariaLabel="Follow symlinks" />
      </Row>
      <Row label="Scan playlist files">
        <Toggle checked={settings.scan_playlist_files ?? true} onChange={(v) => onChange('scan_playlist_files', v)} ariaLabel="Scan playlist files" />
      </Row>
      <div className={styles.field}>
        <label className="field-label" htmlFor="ext">File extensions (comma-separated)</label>
        <input
          id="ext"
          type="text"
          value={(settings.extensions ?? []).join(', ')}
          onChange={(e) => onChange('extensions', e.target.value.split(',').map((s) => s.trim()).filter(Boolean))}
          className={cn('input', 'input--mono')}
        />
      </div>
      <div className={styles.field} style={{ borderBottom: 'none' }}>
        <label className="field-label" htmlFor="cron">Cron schedule</label>
        <input
          id="cron"
          type="text"
          value={settings.scan_schedule ?? ''}
          onChange={(e) => onChange('scan_schedule', e.target.value || null)}
          placeholder="No schedule"
          className={cn('input', 'input--mono')}
        />
      </div>
    </div>
  )
}
