import { http, unwrap } from './client'
import type { CharacterCard, PlayerProfile } from '@/types/genshin'

export interface UIDBinding {
  bound: boolean
  uid?: string
  server?: string
}

/**
 * 聊天页用：**派蒙能不能看到你的角色**（`GET /api/genshin/context-status`，2026-10-12 ✓）。
 *
 * `ready=false` 时的 `reason` 就是"看不到"的原因 ✓，前端据此给出可操作的提示：
 *   - `not_bound`  ⇒ 去账号页绑定 UID ✓
 *   - `no_profile` ⇒ 没有档案也没有历史收录（多为**展柜为空/不公开** ✓）⇒ 提示换回展柜 ✓
 */
export interface PlayerContextStatus {
  bound: boolean
  uid?: string
  /** 聊天是否会带上角色信息（= 已绑定 且「有快照 或 有已收录池」✓；上游临时失败不算 false ✓） */
  ready: boolean
  /** 原因码：'' | 'not_bound' | 'no_profile' */
  reason?: 'not_bound' | 'no_profile' | ''
  hasSnapshot: boolean
  /** 已收录角色数（兜底数据来源 ✓） */
  collectedCount: number
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

  /**
   * 聊天页用：**派蒙现在能不能看到你的角色**（`GET /api/genshin/context-status`，2026-10-12 新增 ✓）。
   *
   * 为什么需要它：此前"聊天没有角色信息"有**三条不同原因**（没绑定 / 上游查询失败 / 展柜为空 ✓），
   * 但对用户是**同一个结果**、且没有任何提示 ✗ —— 用户报障时连自己都判断不出是哪一条 ✓。
   * 进聊天页时调一次，把原因和"下一步去哪做"说清楚 ✓。
   *
   * ⚠️ 服务端只读本地 ✓（绑定 + 快照 + 已收录角色池），**不查 Enka** ✗ ⇒ 便宜，放心调用 ✓。
   */
  contextStatus() {
    return unwrap<PlayerContextStatus>(http.get('/genshin/context-status'))
  },

  /**
   * 绑定 UID 到当前账号（绑定前后端会实查验证）。
   *
   * ⚠️ 2026-10-12 起**界面不再调用它** ✓ —— 查询成功即由**服务端**自动绑定（口径 A ✓，
   * 见 `pages/AccountPage` 的 doQuery）。保留它有两个理由：
   * ① 用户浏览器里缓存的老 bundle 仍在发这个请求 ✗，删了那些页面会"点绑定报 404" ✓；
   * ② 显式绑定本身是合法入口 ✓（后端 `POST /api/genshin/binding` 也没删 ✓），只是当前 UI 用不上 ✓。
   */
  bindUID(uid: string, server = 'cn_gf01') {
    return unwrap<UIDBinding>(http.post('/genshin/binding', { uid, server }))
  },

  /** 解除绑定 */
  unbindUID() {
    return unwrap<UIDBinding>(http.delete('/genshin/binding'))
  },
}
