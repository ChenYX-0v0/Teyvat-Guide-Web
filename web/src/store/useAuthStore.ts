import { create } from 'zustand'
import { authApi, type RegisterOptions } from '@/api/auth'
import { EVENTS, track } from '@/lib/track'

const TOKEN_KEY = 'auth-token'
const NAME_KEY = 'auth-name'

interface AuthState {
  token: string | null
  displayName: string
  /** 是否已用 /auth/me 校验过本地 Token（应用启动时校验一次，避免前端"以为还登录着"） */
  verified: boolean

  // —— 用户须知（免责声明）状态：版本真源在主服务，这里只镜像它 ——
  /** 用户已同意的版本（空 = 从未同意） */
  termsAcceptedVersion: string
  /** 服务端当前生效版本；与 acceptedVersion 不一致 → AppLayout 强制弹窗 */
  termsCurrentVersion: string
  /** 是否已从服务端拿到须知状态（没拿到就不弹，避免页面闪一下） */
  termsLoaded: boolean

  login: (username: string, password: string) => Promise<void>
  /** 手机号注册（成功后自动登录；注册口径见 api/auth.ts 注释） */
  register: (payload: RegisterPayload) => Promise<void>
  logout: () => void
  verify: () => Promise<void>
  /** 记录"我已阅读并同意"（成功后本地版本立即更新，弹窗随之关闭） */
  acceptTerms: () => Promise<void>

  // —— 侧栏用户菜单的两个功能（2026-10-11）——
  /** 改昵称：成功后同步本地 state 与 localStorage，界面（侧栏卡片）立即刷新 ✓ */
  updateDisplayName: (name: string) => Promise<string>
  /**
   * 改密码。⚠️ 成功后**不动本地登录态** ✓：服务端刻意不吊销既有 Token
   * （JWT 里没有密码版本位 ⇒ 见 handler/auth.go 的说明），用户继续用当前会话即可 ✓。
   */
  changePassword: (oldPassword: string, newPassword: string) => Promise<void>

  // —— 注册入口可用性（登录页进页面拉一次；null = 还没拿到/拿不到 ⇒ 不显示注册入口 ✓）——
  registerOptions: RegisterOptions | null
  loadRegisterOptions: () => Promise<void>
}

export interface RegisterPayload {
  phone: string
  code: string
  password: string
  displayName?: string
  /** 邀请码（`requireInviteCode=true` 时必填；服务端会在注册事务里原子占用 ✓） */
  inviteCode?: string
}

function persist(token: string, displayName: string) {
  localStorage.setItem(TOKEN_KEY, token)
  localStorage.setItem(NAME_KEY, displayName)
}

export const useAuthStore = create<AuthState>((set, get) => ({
  token: localStorage.getItem(TOKEN_KEY),
  displayName: localStorage.getItem(NAME_KEY) ?? '',
  verified: false,
  termsAcceptedVersion: '',
  termsCurrentVersion: '',
  termsLoaded: false,
  registerOptions: null,

  async login(username, password) {
    try {
      const { token, displayName } = await authApi.login(username, password)
      persist(token, displayName)
      // 登录成功后须知状态由随后的 verify()（AppLayout 启动时调用）拉取并生效
      set({ token, displayName })
      track(EVENTS.authLogin, { result: 'ok' })
    } catch (e) {
      track(EVENTS.authLogin, { result: 'failed' })
      throw e
    }
  },

  async register({ phone, code, password, displayName, inviteCode }) {
    await authApi.register(phone, code, password, displayName, inviteCode)
    // 注册成功后直接登录：用户名就是手机号（后端约定 ✓）
    //
    // ⚠️ 必须用 `get().login(...)`，**不能**写 `this.login(...)` ✗ ——
    // 组件里的用法是 `const register = useAuthStore((s) => s.register)` 然后 `register({...})`
    // = **裸调用**，`this` 会丢（严格模式下是 undefined）⇒ 运行时崩在
    // 「Cannot read properties of undefined (reading 'login')」✗
    // （2026-09-28 用户真机注册时踩到：账号已建好 ✓，却没能自动登录 ✓，须知也没同意 ✓）。
    await get().login(phone, password)
  },

  /**
   * 拉取注册入口可用性（失败就保持 null = 不显示注册入口 ✓）。
   * 宁可少一个入口，也不给一个"点了才发现不能注册"的死按钮 ✗。
   */
  async loadRegisterOptions() {
    try {
      set({ registerOptions: await authApi.registerConfig() })
    } catch {
      set({ registerOptions: null })
    }
  },

  logout() {
    localStorage.removeItem(TOKEN_KEY)
    localStorage.removeItem(NAME_KEY)
    set({
      token: null,
      displayName: '',
      termsAcceptedVersion: '',
      termsCurrentVersion: '',
      termsLoaded: false,
    })
  },

  /**
   * 用 /auth/me 校验本地 Token 是否还有效（只校验一次），并顺带同步用户须知状态。
   * 失效时服务端返回 401，由 `api/client` 的 401 处理统一清凭据并跳登录页——
   * 这样就不会出现"Token 过期后仍以为自己登录着、实际以匿名身份聊天"的情况。
   */
  async verify() {
    if (get().verified) return
    set({ verified: true })
    if (!localStorage.getItem(TOKEN_KEY)) return
    try {
      const me = await authApi.me()
      set({
        displayName: me.displayName ?? get().displayName,
        termsAcceptedVersion: me.terms?.acceptedVersion ?? '',
        termsCurrentVersion: me.terms?.currentVersion ?? '',
        termsLoaded: true,
      })
    } catch {
      // 登录态失效已由拦截器处理；网络不可达等错误不动本地状态（避免误登出）
    }
  },

  async acceptTerms() {
    const version = get().termsCurrentVersion
    if (!version) throw new Error('未获取到用户须知版本，请刷新页面后重试')
    const res = await authApi.acceptTerms(version)
    set({ termsAcceptedVersion: res.version })
  },

  async updateDisplayName(name) {
    // 以**服务端返回的值**为准（服务端会 TrimSpace ✓，可能和我传进去的不完全一样）
    const { displayName } = await authApi.updateProfile(name)
    localStorage.setItem(NAME_KEY, displayName)
    set({ displayName })
    return displayName
  },

  async changePassword(oldPassword, newPassword) {
    await authApi.changePassword(oldPassword, newPassword)
    // 什么都不改：服务端不吊销旧 Token，当前会话照常有效 ✓（要"改密即踢下线"得先给 JWT 加版本位）
  },
}))

export function getAuthToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}
