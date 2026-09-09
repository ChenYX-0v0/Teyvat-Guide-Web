import { http, unwrap } from './client'
import type { PlayerProfile } from '@/types/genshin'

export interface UIDBinding {
  bound: boolean
  uid?: string
  server?: string
}

export const genshinApi = {
  /** 查询玩家档案（Enka 数据源；默认优先返回本地缓存） */
  player(uid: string, refresh = false) {
    return unwrap<PlayerProfile>(
      http.get('/genshin/player', { params: { uid, refresh } }),
    )
  },

  /** 查询当前登录用户绑定的 UID（未绑定时 bound=false） */
  binding() {
    return unwrap<UIDBinding>(http.get('/genshin/binding'))
  },

  /** 绑定 UID 到当前账号（绑定前后端会实查验证） */
  bindUID(uid: string, server = 'cn_gf01') {
    return unwrap<UIDBinding>(http.post('/genshin/binding', { uid, server }))
  },

  /** 解除绑定 */
  unbindUID() {
    return unwrap<UIDBinding>(http.delete('/genshin/binding'))
  },
}
