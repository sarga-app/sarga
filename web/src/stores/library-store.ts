import { create } from 'zustand'
import { persist } from 'zustand/middleware'

interface LibraryState {
  activeLibraryId: string | null
  setActiveLibraryId: (id: string) => void
}

export const useLibraryStore = create<LibraryState>()(
  persist(
    (set) => ({
      activeLibraryId: null,
      setActiveLibraryId: (id) => set({ activeLibraryId: id }),
    }),
    { name: 'sarga-library' },
  ),
)
