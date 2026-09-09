import axios, { type AxiosError } from 'axios'
import { t } from '@/i18n'
import type { ApiEnvelope } from '@/types'

const baseURL = import.meta.env.VITE_API_BASE_URL ?? '/api'

export const http = axios.create({
  baseURL,
  timeout: 30_000,
  headers: { 'Content-Type': 'application/json' },
})

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
      localStorage.removeItem('auth-token')
      localStorage.removeItem('auth-name')
      if (window.location.pathname !== '/login') {
        window.location.href = '/login'
      }
    }
    const message =
      error.response?.data?.message ?? error.message ?? t('errors.network')
    return Promise.reject(new Error(message))
  },
)

/** 拆箱：后端统一返回 { code, message, data } */
export async function unwrap<T>(promise: Promise<{ data: ApiEnvelope<T> }>): Promise<T> {
  const { data } = await promise
  if (data.code !== 0) {
    throw new Error(data.message || t('errors.requestFailed'))
  }
  return data.data
}
