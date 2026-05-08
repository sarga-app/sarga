import { create } from 'zustand'
import { persist } from 'zustand/middleware'

export type Theme = 'paper' | 'nocturne'

const systemTheme = (): Theme =>
  window.matchMedia('(prefers-color-scheme: dark)').matches ? 'nocturne' : 'paper'

interface UIState {
  theme: Theme
  playerExpanded: boolean
  setTheme: (theme: Theme) => void
  toggleTheme: () => void
  setPlayerExpanded: (v: boolean) => void
}

export const useUIStore = create<UIState>()(
  persist(
    (set, get) => ({
      theme: systemTheme(),
      playerExpanded: false,
      setTheme: (theme) => set({ theme }),
      toggleTheme: () => set({ theme: get().theme === 'paper' ? 'nocturne' : 'paper' }),
      setPlayerExpanded: (playerExpanded) => set({ playerExpanded }),
    }),
    {
      name: 'sarga-ui',
      partialize: (state) => ({ theme: state.theme }),
    },
  ),
)
