import axios, { type AxiosError } from 'axios'
import { t } from '@/i18n'
import type { ApiEnvelope } from '@/types'

const baseURL = import.meta.env.VITE_API_BASE_URL ?? '/api'

export const http = axios.create({
  baseURL,
  timeout: 30_000,
  headers: { 'Content-Type': 'application/json' },
})

http.interceptors.response.use(
  (response) => response,
  (error: AxiosError<ApiEnvelope<unknown>>) => {
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
