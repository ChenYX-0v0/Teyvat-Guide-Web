import { http, unwrap } from './client'
import type { PlayerProfile } from '@/types/genshin'

export const genshinApi = {
  /** 查询玩家档案（Enka 数据源；默认优先返回本地缓存） */
  player(uid: string, refresh = false) {
    return unwrap<PlayerProfile>(
      http.get('/genshin/player', { params: { uid, refresh } }),
    )
  },
}
