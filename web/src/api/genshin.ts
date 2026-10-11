import { http, unwrap } from './client'
import type { CharacterCard, PlayerProfile } from '@/types/genshin'

export interface UIDBinding {
  bound: boolean
  uid?: string
  server?: string
}

/**
 * 已收录角色 = 角色卡 + 收录元信息（2026-10-11）。
 * ⚠️ `inShowcase=false` 表示它**已经不在展柜里** ⇒ 这张卡停在 `lastSeenAt` 那一刻 ✓（可能过时 ✗），
 * 界面上要如实标出"数据截至 …" ✓，别让人以为它是实时的 ✗。
 */
export interface CollectedCharacter extends CharacterCard {
  inShowcase: boolean
  firstSeenAt: string
  lastSeenAt: string
}

/** 已收录角色集合（`GET /api/genshin/characters` 的返回体 ✓） */
export interface CharacterCollection {
  uid: string
  /** 已收录总数（**只增不减** ✓） */
  total: number
  /** 其中当前仍在展柜里的数量（≤12 ✓） */
  showcaseCount: number
  /** 展柜中的在前 ✓，同组内按最近见到倒序 ✓ */
  characters: CollectedCharacter[]
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

  /**
   * 已收录角色（`GET /api/genshin/characters`，2026-10-11 新增）。
   *
   * ⚠️ 与 `player()` 是**两个口径**，别混 ✗：
   *   - `player()`        = **当前展柜**（快照缓存 ✓）—— 用户换展柜它就整体变了 ✓；
   *   - `characters()`    = **历史累积的角色池** —— **只增 / 只改，永不删** ✓。
   *
   * 这样用户反复**轮换展柜**（每次最多 12 位），就能把自己的角色逐步**全部导入** ✓。
   * 代价必须如实呈现：移出展柜的角色不再刷新 ⇒ 它的等级 / 圣遗物停在 `lastSeenAt` ✓（可能过时 ✗）。
   * `uid` 省略时服务端取当前登录用户绑定的 UID ✓。
   */
  characters(uid?: string) {
    return unwrap<CharacterCollection>(
      http.get('/genshin/characters', { params: uid ? { uid } : {} }),
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
