import { create } from 'zustand'
import { persist } from 'zustand/middleware'

interface AdminStore {
  key: string
  isAuthenticated: boolean
  setKey: (key: string) => void
  logout: () => void
}

export const useAdminStore = create<AdminStore>()(
  persist(
    (set) => ({
      key: '',
      isAuthenticated: false,
      setKey: (key: string) => set({ key, isAuthenticated: key.length > 0 }),
      logout: () => set({ key: '', isAuthenticated: false }),
    }),
    {
      name: 'korva-admin',
      // Only persist to sessionStorage — clears when tab closes
      storage: {
        getItem: (name) => {
          const val = sessionStorage.getItem(name)
          return val ? JSON.parse(val) : null
        },
        setItem: (name, value) => {
          sessionStorage.setItem(name, JSON.stringify(value))
        },
        removeItem: (name) => sessionStorage.removeItem(name),
      },
    }
  )
)
