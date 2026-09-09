// Enka 数据源玩家档案类型（与后端 /api/genshin/player 返回结构对齐）

export interface PlayerProfile {
  source: string
  uid: string
  nickname: string
  level: number
  signature: string
  worldLevel: number
  achievements: number
  avatarUrl?: string
  characters: CharacterCard[]
  warnings?: string[]
}

export interface StatInfo {
  name: string
  value: string
}

export interface CharacterCard {
  id: number
  name: string
  element: string
  rarity: number
  level: number
  constellation: number
  fetter: number
  iconUrl?: string
  stats?: StatInfo[]
  weapon?: WeaponInfo
  relics?: RelicInfo[]
}

export interface WeaponInfo {
  name: string
  rarity: number
  level: number
  affix: number
}

export interface RelicInfo {
  name: string
  set?: string // 套装名
  rarity: number
  level: number
  pos: string // EQUIP_BRACER 等，前端映射为部位文案
  mainStat?: string // 主词条（已翻译，如 "生命值 +4780"）
  subStats?: RelicSubStat[]
}

export interface RelicSubStat {
  name: string // 词条名（如 "暴击率"）
  value: string // 当前值（如 "10.5%"）
  count: number // 记录数（含初始获得；强化次数 = count - 1）
}

// 圣遗物部位文案映射
export const relicPosNames: Record<string, string> = {
  EQUIP_BRACER: '生之花',
  EQUIP_NECKLACE: '死之羽',
  EQUIP_SHOES: '时之沙',
  EQUIP_RING: '空之杯',
  EQUIP_DRESS: '理之冠',
}
