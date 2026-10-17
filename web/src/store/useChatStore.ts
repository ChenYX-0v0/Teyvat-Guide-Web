import { create } from 'zustand'
import { chatApi } from '@/api/chat'
import { getAuthToken } from '@/store/useAuthStore'
import { t } from '@/i18n'
import { EVENTS, track } from '@/lib/track'
import { createAbortHandle, isAbortError } from '@/utils/abort'
import type { ChatMessage, ChatSessionItem, ChatSessionMessage, PresetQAItem } from '@/types'

/** 全店共用一个停止句柄：同一时刻只允许一轮流（`loading` 已保证不并发） */
const abortHandle = createAbortHandle()

// 会话管理（需登录；契约见 docs/04-API接口文档.md §3）
// 口径：匿名不持久化——本地只记登录用户的当前会话；消息一律从服务端拉（换设备/清缓存后一致）。
// 匿名（无 Token）已不能提问：对话页展示预设问答（`/api/preset-qa`，不调模型、不消耗 token）。
const SESSION_KEY = 'chat-session-id'

/** 服务端消息 → 视图消息（createdAt 为 RFC3339；解析失败退回当前时间） */
function toMessage(m: ChatSessionMessage): ChatMessage {
  const ts = Date.parse(m.createdAt)
  return {
    id: String(m.id),
    role: m.role,
    content: m.content,
    createdAt: Number.isNaN(ts) ? Date.now() : ts,
  }
}

/** 仅登录用户持久化当前会话；传入空值或未登录即清掉本地记录 */
function persistSession(sessionId?: string) {
  if (!sessionId || !getAuthToken()) {
    localStorage.removeItem(SESSION_KEY)
    return
  }
  localStorage.setItem(SESSION_KEY, sessionId)
}

interface ChatState {
  sessionId?: string
  messages: ChatMessage[]
  loading: boolean
  error?: string
  /** 服务端还有更早的消息（可继续往前加载） */
  hasEarlier: boolean
  /** 已加载消息里最小的服务端 id，作为 beforeId 游标 */
  earliestId?: number
  /** 历史会话列表（仅登录用户；匿名为空数组） */
  sessions: ChatSessionItem[]
  sessionsLoading: boolean
  /**
   * 当前视图所属的登录态。
   * token 变化（退出登录 / 换账号）时清空内存视图，避免上一个账号的对话留在页面上。
   */
  ownerToken: string | null
  /** 预设问答（匿名用户在对话页看到的内容；登录后不再展示） */
  presets: PresetQAItem[]
  presetsLoading: boolean
  loadPresets: () => Promise<void>
  sendMessage: (content: string) => Promise<void>
  /** 停止生成：掐断当前 SSE（不可续），已生成的文字保留并标「已停止生成」 */
  stop: () => void
  /** 新对话：只清当前视图，旧会话仍在服务端（可从列表找回；不提供删除） */
  startNewSession: () => void
  loadSessions: () => Promise<void>
  openSession: (sessionId: string) => Promise<void>
  loadEarlier: () => Promise<void>
  renameSession: (sessionId: string, title: string) => Promise<void>
  /** 删除会话（软删除：C 端不再可见，后台仍可查） */
  deleteSession: (sessionId: string) => Promise<void>
  /** 进入对话页时恢复上次会话（仅登录用户） */
  restore: () => Promise<void>
}

let seq = 0
const nextId = () => `${Date.now()}-${seq++}`

export const useChatStore = create<ChatState>((set, get) => ({
  sessionId: undefined,
  messages: [],
  loading: false,
  error: undefined,
  hasEarlier: false,
  earliestId: undefined,
  sessions: [],
  sessionsLoading: false,
  presets: [],
  presetsLoading: false,
  ownerToken: getAuthToken(),

  async sendMessage(content) {
    const text = content.trim()
    if (!text || get().loading) return

    // 匿名不能提问（/api/chat 已强制登录，见 docs/04 §2）：前端在提问前就拦住
    if (!getAuthToken()) {
      set({ error: t('chat.loginRequired') })
      return
    }

    const session = get().sessionId
    const userMsg: ChatMessage = {
      id: nextId(),
      role: 'user',
      content: text,
      createdAt: Date.now(),
    }
    const pendingMsg: ChatMessage = {
      id: nextId(),
      role: 'assistant',
      content: '',
      createdAt: Date.now(),
      pending: true,
    }

    set((s) => ({
      messages: [...s.messages, userMsg, pendingMsg],
      loading: true,
      error: undefined,
    }))

    // 埋点：提问已发出（不记问题内容——合规 §8 明确不采集对话正文）
    const startedAt = Date.now()
    track(EVENTS.chatMessageSent, { is_first_in_session: !session })

    /** 把增量文本追加到正在生成的这条消息上，并结束"等待中"状态 */
    const appendDelta = (delta: string) => {
      set((s) => ({
        messages: s.messages.map((m) =>
          m.id === pendingMsg.id ? { ...m, content: m.content + delta, pending: false } : m,
        ),
      }))
    }

    const signal = abortHandle.begin()

    try {
      await chatApi.stream(
        { sessionId: session, message: text },
        {
          signal,
          onSession: (sessionId) => {
            set({ sessionId })
            persistSession(sessionId)
          },
          onDelta: appendDelta,
        },
      )

      set((s) => ({
        loading: false,
        messages: s.messages.map((m) =>
          m.id === pendingMsg.id ? { ...m, pending: false, createdAt: Date.now() } : m,
        ),
      }))

      // 回答结束后刷新列表：新会话（首问即建、标题取首问）与消息数都会更新
      if (getAuthToken()) void get().loadSessions()
    } catch (err) {
      // 用户主动停止：不是失败 —— 保留已生成内容、标「已停止生成」，不弹错误、不打失败埋点
      if (isAbortError(err)) {
        set((s) => ({
          loading: false,
          messages: s.messages.map((m) =>
            m.id === pendingMsg.id ? { ...m, pending: false, stopped: true } : m,
          ),
        }))
        return
      }
      // 中断时已生成的内容保留在气泡里，错误文案另起一行提示
      const message = err instanceof Error ? err.message : t('errors.paimonDistracted')
      // 埋点：失败原因粗分（后端也靠这个判断"卡在哪一类失败"）
      track(EVENTS.chatStreamFailed, {
        reason: /超时|timeout/i.test(message)
          ? 'timeout'
          : /晕乎乎/.test(message)
            ? 'fallback'
            : 'network',
        elapsed_ms: Date.now() - startedAt,
      })
      set((s) => ({
        loading: false,
        error: message,
        messages: s.messages.map((m) =>
          m.id === pendingMsg.id ? { ...m, pending: false, error: message } : m,
        ),
      }))
    } finally {
      // 这一轮无论正常 / 失败 / 被停止都清掉句柄，避免下一次 stop 打到已经收尾的流
      abortHandle.end()
    }
  },

  stop() {
    abortHandle.stop()
  },

  startNewSession() {
    persistSession(undefined)
    set({
      sessionId: undefined,
      messages: [],
      error: undefined,
      hasEarlier: false,
      earliestId: undefined,
    })
    track(EVENTS.chatNewSession)
  },

  async loadSessions() {
    if (!getAuthToken()) return
    set({ sessionsLoading: true })
    try {
      const data = await chatApi.listSessions({ pageSize: 50 })
      set({ sessions: data.items, sessionsLoading: false })
    } catch {
      // 列表拉取失败不打断对话：保留上一次的内容
      set({ sessionsLoading: false })
    }
  },

  async loadPresets() {
    set({ presetsLoading: true })
    try {
      const data = await chatApi.presetQa()
      set({ presets: data.items, presetsLoading: false })
    } catch {
      // 取不到不阻塞页面：面板会给出"暂时取不到"的提示
      set({ presets: [], presetsLoading: false })
    }
  },

  async openSession(sessionId) {
    set({ loading: true, error: undefined, hasEarlier: false, earliestId: undefined })
    try {
      const data = await chatApi.sessionMessages(sessionId)
      set({
        sessionId: data.sessionId,
        messages: data.messages.map(toMessage),
        hasEarlier: data.hasMore,
        earliestId: data.messages.length > 0 ? data.messages[0].id : undefined,
        loading: false,
      })
      persistSession(data.sessionId)
      // 埋点：记它被点开时在列表里的位置（不记标题/内容）
      track(EVENTS.chatSessionOpened, {
        index: Math.max(0, get().sessions.findIndex((s) => s.sessionId === sessionId)),
      })
    } catch {
      // 会话不存在/不属于自己/网络异常：回到新对话（只清本地指针，服务端数据不动）
      persistSession(undefined)
      set({
        sessionId: undefined,
        messages: [],
        hasEarlier: false,
        earliestId: undefined,
        loading: false,
      })
    }
  },

  async loadEarlier() {
    const { sessionId, earliestId, loading, hasEarlier } = get()
    if (!sessionId || !earliestId || loading || !hasEarlier) return

    set({ loading: true })
    try {
      const data = await chatApi.sessionMessages(sessionId, { beforeId: earliestId })
      const older = data.messages.map(toMessage)
      set((s) => ({
        messages: [...older, ...s.messages],
        hasEarlier: data.hasMore,
        earliestId: data.messages.length > 0 ? data.messages[0].id : s.earliestId,
        loading: false,
      }))
    } catch (err) {
      set({
        loading: false,
        error: err instanceof Error ? err.message : t('errors.requestFailed'),
      })
    }
  },

  async renameSession(sessionId, title) {
    // 失败时抛出，由列表组件就地提示（不污染对话区的 error）
    const saved = await chatApi.renameSession(sessionId, title)
    set((s) => ({
      sessions: s.sessions.map((it) =>
        it.sessionId === sessionId ? { ...it, title: saved.title } : it,
      ),
    }))
  },

  async deleteSession(sessionId) {
    // 服务端是软删除（数据保留供后台审计），C 端只需把它从自己的列表移除
    await chatApi.deleteSession(sessionId)
    track(EVENTS.chatSessionDeleted)
    set((s) => ({ sessions: s.sessions.filter((it) => it.sessionId !== sessionId) }))

    if (get().sessionId === sessionId) {
      // 删的是当前会话：视图回到新对话（只清本地指针）
      persistSession(undefined)
      set({
        sessionId: undefined,
        messages: [],
        hasEarlier: false,
        earliestId: undefined,
      })
    }
  },

  /**
   * 恢复上次会话：仅登录用户（匿名不持久化，且匿名已不能提问）。
   * 先比对登录态：token 变了（退出/换号）就清空内存视图，再按需恢复；
   * 匿名则改为拉预设问答——对话页展示的就是它。
   */
  async restore() {
    const token = getAuthToken()
    if (token !== get().ownerToken) {
      set({
        ownerToken: token,
        sessionId: undefined,
        messages: [],
        sessions: [],
        hasEarlier: false,
        earliestId: undefined,
        error: undefined,
      })
    }
    if (!token) {
      void get().loadPresets()
      return
    }

    void get().loadSessions()
    const saved = localStorage.getItem(SESSION_KEY)
    if (saved && saved !== get().sessionId) await get().openSession(saved)
  },
}))
