import axios, { type AxiosError } from 'axios'
import { t } from '@/i18n'
import type { ApiEnvelope } from '@/types'

/** API 基地址：与 Vite 代理约定一致（后端地址只在 vite.config.ts 配置） */
export const apiBaseURL = import.meta.env.VITE_API_BASE_URL ?? '/api'

export const http = axios.create({
  baseURL: apiBaseURL,
  timeout: 30_000,
  headers: { 'Content-Type': 'application/json' },
})

/** JWT 请求头：供 fetch 发起的流式请求复用（与下方拦截器同源） */
export function authHeader(): Record<string, string> {
  const token = localStorage.getItem('auth-token')
  return token ? { Authorization: `Bearer ${token}` } : {}
}

/**
 * ApiError 后端业务错误（2026-09-27 新增）。
 * 除了 message，还带上 `code` 与 `data` —— 有些错误必须读 data 才有用：
 * 例如发短信被限速（429）时的 `data.retryAfterSeconds`，前端据此直接接上倒计时 ✓
 * （只给一句"太频繁了"，用户只能自己瞎猜要等多久 ✗）。
 */
export class ApiError extends Error {
  code: number
  data: unknown

  constructor(message: string, code = 0, data: unknown = null) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.data = data
  }
}

/** 登录态失效处理：清凭据并回登录页（axios 拦截器与 fetch 流式请求共用） */
export function handleUnauthorized() {
  localStorage.removeItem('auth-token')
  localStorage.removeItem('auth-name')
  if (window.location.pathname !== '/login') {
    window.location.href = '/login'
  }
}

// 请求自动携带 JWT
http.interceptors.request.use((config) => {
  const token = localStorage.getItem('auth-token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

http.interceptors.response.use(
  (response) => response,
  (error: AxiosError<ApiEnvelope<unknown>>) => {
    // 401：登录态失效 → 清凭据回登录页（登录接口本身的 401 除外）
    if (
      error.response?.status === 401 &&
      !error.config?.url?.includes('/auth/login')
    ) {
      handleUnauthorized()
    }
    const message =
      error.response?.data?.message ?? error.message ?? t('errors.network')
    // 带上 code/data（HTTP 层的错误也走同一种 ApiError ✓，调用方不必区分两种错误形状）
    return Promise.reject(
      new ApiError(message, error.response?.data?.code ?? 0, error.response?.data?.data ?? null),
    )
  },
)

/** 拆箱：后端统一返回 { code, message, data } */
export async function unwrap<T>(promise: Promise<{ data: ApiEnvelope<T> }>): Promise<T> {
  const { data } = await promise
  if (data.code !== 0) {
    throw new ApiError(data.message || t('errors.requestFailed'), data.code, data.data)
  }
  return data.data
}
