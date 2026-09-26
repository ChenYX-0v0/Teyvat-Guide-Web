import { http, unwrap } from './client'

export interface LoginResult {
  token: string
  displayName: string
}

/**
 * 用户须知状态。
 *
 * ⚠️ **版本号的唯一真源在主服务**（`server/internal/service/terms.go` 的 `CurrentTermsVersion`）：
 * 前端不写死版本号，只用服务端下发的 `currentVersion` 与用户已同意的 `acceptedVersion` 比较。
 */
export interface TermsState {
  /** 用户已同意的版本（空 = 从未同意） */
  acceptedVersion: string
  /** 同意时间（ISO 字符串，可空） */
  acceptedAt: string | null
  /** 服务端当前生效版本；与 acceptedVersion 不一致 → 前端强制弹窗 */
  currentVersion: string
}

export interface MeResult {
  id: number
  username: string
  displayName?: string
  genshinUid?: string
  terms?: TermsState
}

export const authApi = {
  login(username: string, password: string) {
    return unwrap<LoginResult>(
      http.post('/auth/login', { username, password }),
    )
  },

  register(username: string, password: string, displayName?: string) {
    return unwrap<{ id: number; username: string }>(
      http.post('/auth/register', { username, password, displayName }),
    )
  },

  /** 当前登录用户 + 用户须知状态（应用启动时校验本地 Token，并决定要不要弹须知） */
  me() {
    return unwrap<MeResult>(http.get('/auth/me'))
  },

  /**
   * 记录"已阅读并同意"。
   * 服务端只接受**当前生效版本**，旧页面提交会被 400 拒绝（见 service/terms.go）。
   */
  acceptTerms(version: string) {
    return unwrap<{ version: string }>(http.post('/auth/terms', { version }))
  },
}
