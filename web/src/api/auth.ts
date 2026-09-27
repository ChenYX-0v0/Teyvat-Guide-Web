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

/**
 * 注册入口可用性（`GET /api/auth/config`，公开接口）。
 *
 * ⚠️ 前端**不能硬编码**"要不要显示注册入口" ✗：是否开放由服务端配置决定
 * （`auth.allow_register` + `sms.enabled`）—— 不公开部署下 `POST /api/auth/register` 直接 403，
 * 硬编码就会出现"点了注册才知道不能注册"的死按钮 ✗。
 */
export interface RegisterOptions {
  allowRegister: boolean
  smsEnabled: boolean
  /** 是否 mock 短信通道（联调用；true 时验证码只在服务端日志里 ✓） */
  smsMock: boolean
  smsCodeTTLSeconds: number
  smsResendAfterSeconds: number
  /** 邀请码功能预留位（当前恒 false ✓） */
  requireInviteCode: boolean
}

/** 发送验证码的结果：前端据此起倒计时 ✓（**响应里不含验证码** ✗） */
export interface SendSmsCodeResult {
  resendAfterSeconds: number
  expiresInSeconds: number
}

export const authApi = {
  login(username: string, password: string) {
    return unwrap<LoginResult>(
      http.post('/auth/login', { username, password }),
    )
  },

  /**
   * 发送注册验证码（限速：同号 60s / 同号日上限 / 同 IP 小时上限；超限返回 429 + retryAfterSeconds）。
   *
   * ⚠️ `inviteCode` 是**必传语义**（服务端 `require_invite_code=true` 时）：
   * 不填 / 不对 ⇒ 服务端**一条短信都不会发** ✓（用户 2026-09-28 口径 ✓），返回 400。
   */
  sendSmsCode(phone: string, inviteCode?: string) {
    return unwrap<SendSmsCodeResult>(http.post('/auth/sms/send', { phone, inviteCode }))
  },

  /** 注册入口可用性（登录页进页面时拉一次 ✓） */
  registerConfig() {
    return unwrap<RegisterOptions>(http.get('/auth/config'))
  },

  /**
   * 手机号注册（2026-09-27）：手机号 + 短信验证码 + 设置密码。
   * 成功后**不会自动登录**（注册与登录是两步 ✓），由 store 串起来。
   */
  register(phone: string, code: string, password: string, displayName?: string, inviteCode?: string) {
    return unwrap<{ id: number; username: string; displayName: string }>(
      // 注册时会**原子占用**邀请码（用过一次即作废 ✓，见 server/internal/repository/invite.go）
      http.post('/auth/register', { phone, code, password, displayName, inviteCode }),
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
