import { create } from 'zustand'

interface AuthState {
  passkey: string | null
  setPasskey: (key: string) => void
  logout: () => void
  isAuthenticated: () => boolean
}

export const useAuth = create<AuthState>((set, get) => ({
  passkey: localStorage.getItem('passkey'),
  setPasskey: (key) => {
    localStorage.setItem('passkey', key)
    set({ passkey: key })
  },
  logout: () => {
    localStorage.removeItem('passkey')
    set({ passkey: null })
  },
  isAuthenticated: () => !!get().passkey,
}))
