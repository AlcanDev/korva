import { create } from 'zustand'

interface AppState {
  activeProject: string
  activeTeam: string
  sidebarOpen: boolean
  setActiveProject: (project: string) => void
  setActiveTeam: (team: string) => void
  toggleSidebar: () => void
}

export const useAppStore = create<AppState>((set) => ({
  activeProject: '',
  activeTeam: '',
  sidebarOpen: true,
  setActiveProject: (project) => set({ activeProject: project }),
  setActiveTeam: (team) => set({ activeTeam: team }),
  toggleSidebar: () => set((state) => ({ sidebarOpen: !state.sidebarOpen })),
}))
