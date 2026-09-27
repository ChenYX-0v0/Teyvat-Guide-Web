import { http, unwrap } from './client'
import type { PlayerProfile } from '@/types/genshin'

export interface UIDBinding {
  bound: boolean
  uid?: string
  server?: string
}

export const genshinApi = {
  /**
   * 查询玩家档案（Enka 数据源；服务端优先返回本地快照 ✓）。
   *
   * ⚠️ 2026-09-28：**前端不再传 `refresh`** —— 账号页去掉了「强制刷新（绕过本地缓存）」入口 ✓。
   * 服务端参数**照旧保留** ✓，原因有二：① 运维核对资产时要用
   * `GET /api/genshin/player?uid=…&refresh=true`（见 `web/14` 的配方）；
   * ② 用户浏览器里可能缓存着**旧 bundle**，它仍在发这个参数 ⇒ 不能删后端支持 ✗。
   */
  player(uid: string) {
    return unwrap<PlayerProfile>(http.get('/genshin/player', { params: { uid } }))
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
