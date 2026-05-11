import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { Library, Plus } from 'lucide-react'
import { listLibraries } from '@/services/catalog'
import { api } from '@/services/api'
import { useAuthStore } from '@/features/auth/auth-store'
import { useLibraryStore } from '@/stores/library-store'
import { PageLoader } from '@/components/ui/page-loader'
import { Modal } from '@/components/ui/modal'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/cn'
import styles from './library-page.module.css'
import type { AddLibraryModalProps } from './library-page.types'

function authHeaders() {
  const token = useAuthStore.getState().accessToken
  return token ? { Authorization: `Bearer ${token}` } : {}
}

async function createLibrary(name: string, paths: string[]) {
  const { data, error } = await api.POST('/libraries', {
    headers: authHeaders(),
    body: { name, paths },
  })
  if (error) throw error
  return data
}

function AddLibraryModal({ onClose }: AddLibraryModalProps) {
  const [name, setName] = useState('')
  const [paths, setPaths] = useState('')
  const queryClient = useQueryClient()
  const { setActiveLibraryId } = useLibraryStore()

  const { mutate, isPending, isError, error } = useMutation({
    mutationFn: () =>
      createLibrary(
        name,
        paths.split('\n').map((p) => p.trim()).filter(Boolean),
      ),
    onSuccess: (lib) => {
      queryClient.invalidateQueries({ queryKey: ['libraries'] })
      if (lib?.id) setActiveLibraryId(lib.id)
      onClose()
    },
  })

  const disabled = isPending || !name.trim() || !paths.trim()

  return (
    <Modal
      open
      onClose={onClose}
      title="Add library"
      size="sm"
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>Cancel</Button>
          <Button variant="primary" onClick={() => mutate()} disabled={disabled}>
            {isPending ? 'Creating…' : 'Create library'}
          </Button>
        </>
      }
    >
      <div style={{ marginBottom: 'var(--s-4)' }}>
        <label className="field-label" htmlFor="lib-name">Library name</label>
        <input
          id="lib-name"
          type="text"
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="Main Library"
          className="input"
        />
      </div>
      <div style={{ marginBottom: 'var(--s-3)' }}>
        <label className="field-label" htmlFor="lib-paths">Paths (one per line)</label>
        <textarea
          id="lib-paths"
          value={paths}
          onChange={(e) => setPaths(e.target.value)}
          placeholder="/music/library"
          rows={4}
          className={cn('input', 'input--textarea')}
        />
      </div>

      {isError && (
        <p style={{ color: 'var(--danger)', fontSize: 13, margin: 0 }}>
          {(error as { message?: string })?.message ?? 'Something went wrong'}
        </p>
      )}
    </Modal>
  )
}

export function LibraryPage() {
  const [addOpen, setAddOpen] = useState(false)

  const { data, isLoading } = useQuery({
    queryKey: ['libraries'],
    queryFn: listLibraries,
  })

  const libraries = data?.libraries ?? []

  return (
    <div>
      <div className={styles.header}>
        <span className="eyebrow">Library</span>
        <Button variant="primary" size="sm" onClick={() => setAddOpen(true)}>
          <Plus size={14} strokeWidth={2} />
          Add library
        </Button>
      </div>

      {isLoading ? (
        <PageLoader />
      ) : libraries.length === 0 ? (
        <div className={styles.empty}>
          <Library size={48} strokeWidth={1} color="var(--fg-4)" />
          <div>
            <h2 className={styles.emptyHeading}>Your music, organized your way</h2>
            <p className={styles.emptyBody}>
              Add a library to point Sarga at your music folders. It will scan, identify, and
              organize your collection — bit-perfect, no cloud required.
            </p>
            <Button variant="primary" onClick={() => setAddOpen(true)}>
              <Plus size={16} strokeWidth={2} />
              Add your first library
            </Button>
          </div>
        </div>
      ) : (
        <div className={styles.cards}>
          {libraries.map((lib) => (
            <div key={lib.id} className={styles.card}>
              <div className={styles.cardRow}>
                <div className={styles.cardContent}>
                  <h3 className={styles.cardName}>{lib.name}</h3>
                  <div className={styles.paths}>
                    {lib.paths.map((p) => (
                      <span key={p} className={styles.path}>{p}</span>
                    ))}
                  </div>
                </div>
                <Link to="/albums" className={styles.browseLink}>Browse →</Link>
              </div>
            </div>
          ))}
        </div>
      )}

      {addOpen && <AddLibraryModal onClose={() => setAddOpen(false)} />}
    </div>
  )
}
