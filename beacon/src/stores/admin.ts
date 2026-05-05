import { create } from 'zustand'
import { persist } from 'zustand/middleware'

interface AdminStore {
  key: string
  sessionToken: string
  authMode: 'key' | 'session'
  isAuthenticated: boolean
  setKey: (key: string) => void
  setSessionToken: (token: string) => void
  logout: () => void
}

export const useAdminStore = create<AdminStore>()(
  persist(
    (set) => ({
      key: '',
      sessionToken: '',
      authMode: 'key' as const,
      isAuthenticated: false,
      setKey: (key: string) =>
        set({ key, sessionToken: '', authMode: 'key', isAuthenticated: key.length > 0 }),
      setSessionToken: (token: string) =>
        set({ sessionToken: token, key: '', authMode: 'session', isAuthenticated: token.length > 0 }),
      logout: () => set({ key: '', sessionToken: '', authMode: 'key', isAuthenticated: false }),
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
