import { Toggle } from '@/components/ui/toggle'
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

export function MetadataTab({ settings, onChange }: Props) {
  const threshold = settings.match_confidence_threshold ?? 0.85
  return (
    <div>
      <Row label="MusicBrainz lookup">
        <Toggle checked={settings.enable_musicbrainz ?? true} onChange={(v) => onChange('enable_musicbrainz', v)} ariaLabel="MusicBrainz lookup" />
      </Row>
      <Row label="AcoustID fingerprinting">
        <Toggle checked={settings.enable_acoustid ?? true} onChange={(v) => onChange('enable_acoustid', v)} ariaLabel="AcoustID fingerprinting" />
      </Row>
      <Row label="Discogs lookup">
        <Toggle checked={settings.enable_discogs ?? true} onChange={(v) => onChange('enable_discogs', v)} ariaLabel="Discogs lookup" />
      </Row>
      <div className={styles.field} style={{ borderBottom: 'none' }}>
        <label className="field-label" htmlFor="thresh">
          Match confidence threshold:{' '}
          <strong style={{ color: 'var(--fg-1)' }}>{Math.round(threshold * 100)}%</strong>
        </label>
        <input
          id="thresh"
          type="range"
          min={0}
          max={1}
          step={0.01}
          value={threshold}
          onChange={(e) => onChange('match_confidence_threshold', parseFloat(e.target.value))}
          style={{ width: '100%', accentColor: 'var(--accent)' }}
        />
      </div>
    </div>
  )
}
