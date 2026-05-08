import { create } from 'zustand'

type Theme = 'paper' | 'nocturne'

interface UIState {
  theme: Theme
  playerExpanded: boolean
  setTheme: (theme: Theme) => void
  setPlayerExpanded: (v: boolean) => void
}

export const useUIStore = create<UIState>()((set) => ({
  theme: 'paper',
  playerExpanded: false,
  setTheme: (theme) => set({ theme }),
  setPlayerExpanded: (playerExpanded) => set({ playerExpanded }),
}))
