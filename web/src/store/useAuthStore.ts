import { create } from 'zustand'
import { authApi } from '@/api/auth'

const TOKEN_KEY = 'auth-token'
const NAME_KEY = 'auth-name'

interface AuthState {
  token: string | null
  displayName: string
  login: (username: string, password: string) => Promise<void>
  register: (username: string, password: string, displayName?: string) => Promise<void>
  logout: () => void
}

function persist(token: string, displayName: string) {
  localStorage.setItem(TOKEN_KEY, token)
  localStorage.setItem(NAME_KEY, displayName)
}

export const useAuthStore = create<AuthState>((set) => ({
  token: localStorage.getItem(TOKEN_KEY),
  displayName: localStorage.getItem(NAME_KEY) ?? '',

  async login(username, password) {
    const { token, displayName } = await authApi.login(username, password)
    persist(token, displayName)
    set({ token, displayName })
  },

  async register(username, password, displayName) {
    await authApi.register(username, password, displayName)
    // 注册成功后直接登录
    await this.login(username, password)
  },

  logout() {
    localStorage.removeItem(TOKEN_KEY)
    localStorage.removeItem(NAME_KEY)
    set({ token: null, displayName: '' })
  },
}))

export function getAuthToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}
