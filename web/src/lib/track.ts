/**
 * 埋点上报（真源：shared-docs/07-前端埋点与使用统计.md）
 *
 * 三条铁律（与后端 `service/usage.go` 对称，改这里前先读一遍）：
 *  1. **永不阻塞主流程**：不 await、失败静默丢弃、不重试、不弹任何提示；
 *  2. **生产才发**：本地 dev 只 `console.debug`；服务端可整体关停（`usage.enabled`）；
 *  3. **不带身份标识**：D1 定案（2026-09-26）不采集匿名标识 —— 只带"是否登录"布尔，
 *     不采集对话内容、密码、UID 原值、设备标识。
 *
 * 加新事件必须先在后端白名单登记（`server/internal/service/usage.go` 的 `usageEventProps`），
 * 否则会被后端丢弃 —— 事件名一律走下面的 `EVENTS` 常量，禁止在组件里写字符串。
 */

export const EVENTS = {
  pageView: 'page_view',
  pageLeave: 'page_leave',
  homeCtaClick: 'home_cta_click',
  chatMessageSent: 'chat_message_sent',
  chatStreamFailed: 'chat_stream_failed',
  chatNewSession: 'chat_new_session',
  chatSessionOpened: 'chat_session_opened',
  chatSessionDeleted: 'chat_session_deleted',
  chatPresetExpanded: 'chat_preset_expanded',
  accountUidQuery: 'account_uid_query',
  accountUidBind: 'account_uid_bind',
  accountUnbind: 'account_unbind',
  accountAnalysisOpen: 'account_analysis_open',
  accountAnalysisAuto: 'account_analysis_auto',
  accountAnalysisRerun: 'account_analysis_rerun',
  accountAnalysisEnd: 'account_analysis_end',
  authLogin: 'auth_login',
  webError: 'web_error',
} as const

type Props = Record<string, string | number | boolean>

type Item = {
  event: string
  props: Props
  page: string
  platform: string
  clientTs: number
}

/** 队列与发送节奏（§4.2） */
const FLUSH_MS = 5000
const MAX_QUEUE = 20
/** 停留时长上下截断（D2）：<1s 视为误触、>30min 记为超长（后端会再截一次） */
const MIN_DWELL_MS = 1000
const MAX_DWELL_MS = 30 * 60 * 1000

/** 只有生产构建才真的发；dev/测试只打 console */
const enabled = import.meta.env.PROD

const apiBase = import.meta.env.VITE_API_BASE_URL ?? '/api'

let queue: Item[] = []
let timer: number | undefined

/** 路径 → 页面 key（与后端 usagePages 白名单一致） */
export function pageKey(path: string = window.location.pathname): string {
  if (path.startsWith('/chat')) return 'chat'
  if (path.startsWith('/account')) return 'account'
  if (path.startsWith('/wiki')) return 'wiki'
  if (path.startsWith('/login')) return 'login'
  return 'home'
}

function platform(): string {
  return /Mobi|Android|iPhone|iPad|iPod/i.test(navigator.userAgent) ? 'mobile' : 'desktop'
}

function isLoggedIn(): boolean {
  return Boolean(localStorage.getItem('auth-token'))
}

/** 唯一入口：记一条事件（不 await、不抛错） */
export function track(event: string, props: Props = {}): void {
  if (!enabled) {
    if (import.meta.env.DEV) console.debug('[track]', event, props)
    return
  }
  try {
    queue.push({
      event,
      props,
      page: pageKey(),
      platform: platform(),
      clientTs: Date.now(),
    })
    if (queue.length >= MAX_QUEUE) flush()
    else if (timer === undefined) timer = window.setTimeout(flush, FLUSH_MS)
  } catch {
    /* 原则 1：埋点出任何问题都不影响主流程 */
  }
}

/** 立即发送（幂等；页面隐藏 / 路由变化时调用） */
export function flush(): void {
  if (timer !== undefined) {
    window.clearTimeout(timer)
    timer = undefined
  }
  if (queue.length === 0) return
  const items = queue
  queue = []
  const url = `${apiBase}/events`
  const body = JSON.stringify({ events: items })

  // 优先 fetch + keepalive：它能带 Authorization（保住 user_id），且卸载时也会把请求发完
  try {
    void fetch(url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        ...(isLoggedIn() ? { Authorization: `Bearer ${localStorage.getItem('auth-token')}` } : {}),
      },
      body,
      keepalive: true,
    }).catch(() => {
      /* 静默丢弃 */
    })
    return
  } catch {
    /* 落到 sendBeacon */
  }
  try {
    // sendBeacon 兜底：发得出去但不带鉴权头（这类事件会记成匿名，可接受）
    navigator.sendBeacon?.(url, new Blob([body], { type: 'application/json' }))
  } catch {
    /* 静默丢弃 */
  }
}

// ── 页面浏览与停留（D2：停留时长照采）──

let currentPath = ''
let enterAt = 0
let hidden = false

function reportLeave(): void {
  if (!currentPath) return
  const dwell = Date.now() - enterAt
  if (dwell >= MIN_DWELL_MS) {
    track(EVENTS.pageLeave, { dwell_ms: Math.min(dwell, MAX_DWELL_MS) })
  }
}

/** 路由变化时调用（AppLayout 里监听 location）：上报上一次停留 + 本次浏览 */
export function trackRouteChange(path: string): void {
  // 同一路径不重复上报：initTracking 已经报过首次进入，且 StrictMode 下 effect 会跑两遍
  if (path === currentPath) return
  if (!hidden) reportLeave()
  track(EVENTS.pageView, {
    referrer_page: currentPath ? pageKey(currentPath) : '',
    is_logged_in: isLoggedIn(),
  })
  currentPath = path
  enterAt = Date.now()
  flush() // 路由变化是天然的 flush 点
}

/** 应用启动时调用一次：接管"切后台/切回来"与前端异常 */
export function initTracking(): void {
  trackRouteChange(window.location.pathname)

  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'hidden') {
      // 切后台 = 一次离开：先记停留，再把队列发出去（sendBeacon 能救回这部分数据）
      if (!hidden) reportLeave()
      hidden = true
      flush()
    } else {
      hidden = false
      enterAt = Date.now() // 回到前台重新计时，但不重复记 page_view
    }
  })

  // 前端异常（只存哈希，不存原文）
  window.addEventListener('error', (e) => {
    track(EVENTS.webError, { message_hash: hash(String(e.message || e.type)) })
  })
  window.addEventListener('unhandledrejection', () => {
    track(EVENTS.webError, { message_hash: hash('unhandledrejection') })
  })
}

/** 极简 FNV-1a 哈希：埋点只需要"同一错误能对上"，不需要还原原文 */
function hash(s: string): string {
  let h = 0x811c9dc5
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i)
    h = Math.imul(h, 0x01000193)
  }
  return (h >>> 0).toString(36)
}
