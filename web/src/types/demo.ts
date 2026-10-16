// 游客演示模式类型（与后端 `GET /api/demo/showcase` 对齐，见 server/internal/dto/demo.go）
//
// 口径：演示页**不连 AI 服务** —— 账号头、角色卡、「分析」全部是库里预置内容
// （`demo_profiles` / `demo_characters`），改文案改库即可、不用发版 ✓。
// 因为 `card` 与真实档案**同构**（types/genshin.ts 的 CharacterCard ✓），
// 前端可以直接把它当展柜角色卡渲染 ✓ —— 面板 / 圣遗物 / 评分组件全部复用现有组件 ✓。

import type { CharacterCard } from './genshin'

/** 深渊概要：演示数据没配时为 undefined ⇒ 前端整块不渲染 ✓（"没有" ≠ "0 层"） */
export interface DemoAbyssSummary {
  floor: number
  room: number
  stars: number
}

/** 演示账号头（模拟旅行者，**不是真账号**：没有游戏 UID） */
export interface DemoProfileInfo {
  nickname: string
  signature: string
  level: number
  worldLevel: number
  achievements: number
  abyss?: DemoAbyssSummary
}

/** 演示角色 = 定位标签 + 与真实档案同构的角色卡 + 预置「分析」 */
export interface DemoCharacterItem {
  charKey: string
  sort: number
  name: string
  element: string
  rarity: number
  role: string
  card: CharacterCard
  analysis: string
}

export interface DemoShowcase {
  /** 库里没配演示档案时为 null ⇒ 页面退回昵称等文案兜底 ✓ */
  profile: DemoProfileInfo | null
  characters: DemoCharacterItem[]
  /** 演示内容提示语（措辞随产品/合规要求会改 ⇒ 放库可运营，前端不写死 ✓） */
  notice: string
}
