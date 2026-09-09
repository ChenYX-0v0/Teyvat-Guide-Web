import { create } from 'zustand'
import { authApi, type UserRole } from '@/api/auth'

const TOKEN_KEY = 'auth-token'
const NAME_KEY = 'auth-name'
const ROLE_KEY = 'auth-role'

interface AuthState {
  token: string | null
  displayName: string
  role: UserRole | null
  login: (username: string, password: string) => Promise<void>
  register: (username: string, password: string, displayName?: string) => Promise<void>
  logout: () => void
}

function persist(token: string, displayName: string, role: UserRole) {
  localStorage.setItem(TOKEN_KEY, token)
  localStorage.setItem(NAME_KEY, displayName)
  localStorage.setItem(ROLE_KEY, role)
}

export const useAuthStore = create<AuthState>((set) => ({
  token: localStorage.getItem(TOKEN_KEY),
  displayName: localStorage.getItem(NAME_KEY) ?? '',
  role: (localStorage.getItem(ROLE_KEY) as UserRole | null) ?? null,

  async login(username, password) {
    const { token, displayName, role } = await authApi.login(username, password)
    persist(token, displayName, role)
    set({ token, displayName, role })
  },

  async register(username, password, displayName) {
    await authApi.register(username, password, displayName)
    // 注册成功后直接登录
    await this.login(username, password)
  },

  logout() {
    localStorage.removeItem(TOKEN_KEY)
    localStorage.removeItem(NAME_KEY)
    localStorage.removeItem(ROLE_KEY)
    set({ token: null, displayName: '', role: null })
  },
}))

export function getAuthToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function isAdminRole(role: string | null | undefined): boolean {
  return role === 'admin'
}
