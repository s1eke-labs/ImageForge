import { create } from 'zustand'
import { api } from '../api/client.js'
import { clearSession, getSession, setSession } from './authSession.js'

const session = getSession()

export const useAuthStore = create((set) => ({
  token: session.token,
  username: session.username,
  async login(username, password) {
    const { token } = await api.post('auth/login', { json: { username, password } }).json()
    setSession(token, username)
    set({ token, username })
  },
  logout() {
    clearSession()
    set({ token: '', username: '' })
  },
}))
