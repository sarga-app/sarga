import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getLibrarySettings, putLibrarySettings } from '@/services/catalog'
import { Modal } from '@/components/ui/modal'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/cn'
import styles from './settings-modal.module.css'
import { ScanningTab } from './scanning-tab'
import { MetadataTab } from './metadata-tab'
import { ArtworkTab } from './artwork-tab'
import type { LibrarySettings, SettingsTab, SettingsModalProps } from './settings-modal.types'

const TABS: { id: SettingsTab; label: string }[] = [
  { id: 'scanning', label: 'Scanning' },
  { id: 'metadata', label: 'Metadata' },
  { id: 'artwork',  label: 'Artwork'  },
]

export function SettingsModal({ libraryId, onClose }: SettingsModalProps) {
  const [tab, setTab] = useState<SettingsTab>('scanning')
  const queryClient = useQueryClient()

  const { data: settings, isLoading } = useQuery({
    queryKey: ['settings', libraryId],
    queryFn: () => getLibrarySettings(libraryId),
  })

  const [local, setLocal] = useState<Partial<LibrarySettings>>({})
  const merged: Partial<LibrarySettings> = { ...settings, ...local }

  const { mutate: save, isPending: saving } = useMutation({
    mutationFn: () => putLibrarySettings(libraryId, merged as Record<string, unknown>),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['settings', libraryId] })
      setLocal({})
    },
  })

  const set = <K extends keyof LibrarySettings>(key: K, value: LibrarySettings[K]) => {
    setLocal((prev) => ({ ...prev, [key]: value }))
  }

  const isDirty = Object.keys(local).length > 0

  return (
    <Modal
      open
      onClose={onClose}
      title="Library settings"
      size="md"
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>Cancel</Button>
          <Button variant="primary" onClick={() => save()} disabled={saving || !isDirty}>
            {saving ? 'Saving…' : 'Save'}
          </Button>
        </>
      }
    >
      <div className={styles.tabs}>
        {TABS.map(({ id, label }) => (
          <button
            key={id}
            type="button"
            onClick={() => setTab(id)}
            className={cn(styles.tabBtn, tab === id && styles.tabActive)}
          >
            {label}
          </button>
        ))}
      </div>

      <div className={styles.body}>
        {isLoading ? (
          <p style={{ color: 'var(--fg-3)', fontSize: 14 }}>Loading…</p>
        ) : tab === 'scanning' ? (
          <ScanningTab settings={merged} onChange={set} />
        ) : tab === 'metadata' ? (
          <MetadataTab settings={merged} onChange={set} />
        ) : (
          <ArtworkTab settings={merged} onChange={set} />
        )}
      </div>
    </Modal>
  )
}
