/**
 * 「养成度体检」自动触发门闩（真源：teyvat-docs/web/13-账号页悬浮AI分析（角色养成度）设计.md §4）
 *
 * 三件事：
 *  1. 记「哪些角色已经自动体检过」——按 UID 分桶；**只记 characterId + 时间戳**，
 *     不记结论内容（结论在会话里，见 §5）；
 *  2. 解绑 / 换绑 UID 后**清空整个桶**（换了账号就该从零开始）；
 *     ⚠️ 2026-09-28：原「强制刷新档案成功后清空」已随账号页去掉该入口而取消 ✓ ——
 *     现在想重做体检走面板里的**手动「重新分析」**（本就不受"只自动一次"限制 ✓）。
 *  3. 当日自动体检次数软上限（纯前端计数；服务端配额仍是硬闸）。
 *
 * 为什么放 localStorage（D2 定案）：不新增后端表、不采集新的个人数据。
 * 代价明确且可接受：清缓存 / 换设备会重来一次；跨设备的刷新不会重置计数。
 */

/** 每个 UID 一个桶：characterId → 首次通过时间戳（毫秒） */
const DONE_PREFIX = 'teyvat:analysis:done:'
/** 分析会话 id（每个 UID 一条独立会话，见 D1） */
const SESSION_PREFIX = 'teyvat:analysis:session:'
/** 当日自动体检次数（纯前端软上限） */
const DAILY_KEY = 'teyvat:analysis:daily'

/** 每日自动体检上限：防止"反复强制刷新 → 反复自动体检"把额度烧掉 */
const DAILY_LIMIT = 20
/** 同一角色的自动体检连续失败次数上限（本次页面会话内不再自动重试，手动「重新分析」不受限） */
const MAX_AUTO_FAILURES = 2

type DoneMap = Record<string, number>

function doneKey(uid: string): string {
  return DONE_PREFIX + uid
}

function readDone(uid: string): DoneMap {
  if (!uid) return {}
  try {
    const raw = localStorage.getItem(doneKey(uid))
    if (!raw) return {}
    const parsed: unknown = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object') return {}
    return parsed as DoneMap
  } catch {
    // 脏数据 / 隐私模式：当作"没体检过"，不影响主流程
    return {}
  }
}

function writeDone(uid: string, map: DoneMap): void {
  if (!uid) return
  try {
    localStorage.setItem(doneKey(uid), JSON.stringify(map))
  } catch {
    /* 配额满 / 隐私模式：静默降级为"每次都问"，不报错、不阻塞 */
  }
}

/** 该角色是否已自动体检过 */
export function isAnalyzed(uid: string, characterId: number): boolean {
  return readDone(uid)[String(characterId)] !== undefined
}

/** 该 UID 已体检过的角色数（埋点 is_first_char 用） */
export function analyzedCount(uid: string): number {
  return Object.keys(readDone(uid)).length
}

/** 记一次"已体检"——**只在流正常结束（done 帧）后调用** */
export function markAnalyzed(uid: string, characterId: number): void {
  const map = readDone(uid)
  map[String(characterId)] = Date.now()
  writeDone(uid, map)
}

/**
 * 清空该 UID 的体检记录（保留会话 → 历史报告仍可回看）。
 * 调用时机：① 强制刷新档案成功 ② 解绑 / 换绑 UID。
 */
export function resetAnalyzed(uid: string): void {
  if (!uid) return
  try {
    localStorage.removeItem(doneKey(uid))
  } catch {
    /* 静默 */
  }
}

/** 分析会话 id 的持久化（每个 UID 一条） */
export function readSessionId(uid: string): string | undefined {
  if (!uid) return undefined
  try {
    return localStorage.getItem(SESSION_PREFIX + uid) ?? undefined
  } catch {
    return undefined
  }
}

export function writeSessionId(uid: string, sessionId: string): void {
  if (!uid) return
  try {
    localStorage.setItem(SESSION_PREFIX + uid, sessionId)
  } catch {
    /* 静默 */
  }
}

export function clearSessionId(uid: string): void {
  if (!uid) return
  try {
    localStorage.removeItem(SESSION_PREFIX + uid)
  } catch {
    /* 静默 */
  }
}

/** 当日是否还能自动体检 */
export function canRunAutoToday(): boolean {
  return readDaily().n < DAILY_LIMIT
}

/** 记一次自动体检的**尝试**（发出即记，避免失败重试把日上限绕过去） */
export function markAutoToday(): void {
  const today = todayKey()
  const cur = readDaily()
  const next = cur.date === today ? { date: today, n: cur.n + 1 } : { date: today, n: 1 }
  try {
    localStorage.setItem(DAILY_KEY, JSON.stringify(next))
  } catch {
    /* 静默 */
  }
}

function todayKey(): string {
  const d = new Date()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${d.getFullYear()}-${m}-${day}`
}

function readDaily(): { date: string; n: number } {
  const today = todayKey()
  try {
    const raw = localStorage.getItem(DAILY_KEY)
    if (!raw) return { date: today, n: 0 }
    const parsed = JSON.parse(raw) as { date?: string; n?: number }
    // 跨天自动归零
    if (parsed.date !== today) return { date: today, n: 0 }
    return { date: today, n: typeof parsed.n === 'number' ? parsed.n : 0 }
  } catch {
    return { date: today, n: 0 }
  }
}

// ── 失败计数：只在内存里 ──
// 「本次页面会话内不再自动重试」——退出/刷新页面即失效，语义正好对应内存。

const failures = new Map<string, number>()

function failKey(uid: string, characterId: number): string {
  return `${uid}:${characterId}`
}

/** 该角色的自动体检是否已连续失败到上限 */
export function autoFailuresExceeded(uid: string, characterId: number): boolean {
  return (failures.get(failKey(uid, characterId)) ?? 0) >= MAX_AUTO_FAILURES
}

/** 记一次自动体检失败（429 配额 / 网络 / 流中断） */
export function markAutoFailure(uid: string, characterId: number): void {
  const key = failKey(uid, characterId)
  failures.set(key, (failures.get(key) ?? 0) + 1)
}
