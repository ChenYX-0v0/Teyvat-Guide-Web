import { http, unwrap } from './client'

export type UserRole = 'admin' | 'user'

export interface LoginResult {
  token: string
  displayName: string
  role: UserRole
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
}
