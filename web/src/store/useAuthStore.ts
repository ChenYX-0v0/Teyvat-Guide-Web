import { create } from 'zustand'
import { authApi } from '@/api/auth'

const TOKEN_KEY = 'auth-token'
const NAME_KEY = 'auth-name'

interface AuthState {
  token: string | null
  displayName: string
  /** 是否已用 /auth/me 校验过本地 Token（应用启动时校验一次，避免前端"以为还登录着"） */
  verified: boolean
  login: (username: string, password: string) => Promise<void>
  register: (username: string, password: string, displayName?: string) => Promise<void>
  logout: () => void
  verify: () => Promise<void>
}

function persist(token: string, displayName: string) {
  localStorage.setItem(TOKEN_KEY, token)
  localStorage.setItem(NAME_KEY, displayName)
}

export const useAuthStore = create<AuthState>((set, get) => ({
  token: localStorage.getItem(TOKEN_KEY),
  displayName: localStorage.getItem(NAME_KEY) ?? '',
  verified: false,

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

  /**
   * 用 /auth/me 校验本地 Token 是否还有效（只校验一次）。
   * 失效时服务端返回 401，由 `api/client` 的 401 处理统一清凭据并跳登录页——
   * 这样就不会出现"Token 过期后仍以为自己登录着、实际以匿名身份聊天"的情况。
   */
  async verify() {
    if (get().verified) return
    set({ verified: true })
    if (!localStorage.getItem(TOKEN_KEY)) return
    try {
      await authApi.me()
    } catch {
      // 登录态失效已由拦截器处理；网络不可达等错误不动本地状态（避免误登出）
    }
  },
}))

export function getAuthToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}
