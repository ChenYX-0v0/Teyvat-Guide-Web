import { http, unwrap } from './client'

export interface LoginResult {
  token: string
  displayName: string
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

  /** 当前登录用户（应用启动时校验本地 Token 是否仍有效） */
  me() {
    return unwrap<{ id: number; username: string }>(http.get('/auth/me'))
  },
}
