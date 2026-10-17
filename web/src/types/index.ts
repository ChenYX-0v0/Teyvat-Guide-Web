export type Role = 'user' | 'assistant' | 'system'

export interface ChatMessage {
  id: string
  role: Role
  content: string
  createdAt: number
  pending?: boolean
  error?: string
  /** 用户主动停止生成（内容保留，不是错误） */
  stopped?: boolean
}

export interface ChatRequest {
  sessionId?: string
  message: string
  stream?: boolean
}

export interface ChatResponse {
  sessionId: string
  reply: string
  tokens: number
  elapsedMs: number
}

/** 流式对话的 done 帧载荷 */
export interface ChatStreamDone {
  sessionId: string
  tokens: number
  elapsedMs: number
}

/** 流式对话回调（帧格式见 docs/04-API接口文档.md §2.1） */
export interface ChatStreamHandlers {
  /** 会话建立帧：流中断时前端也能保留会话归属 */
  onSession?: (sessionId: string) => void
  /** 增量文本帧，可能出现多次 */
  onDelta: (text: string) => void
  /** 正常结束帧 */
  onDone?: (data: ChatStreamDone) => void
  /** 取消流（如组件卸载） */
  signal?: AbortSignal
}

/** 会话列表项（需登录；契约见 docs/04-API接口文档.md §3） */
export interface ChatSessionItem {
  sessionId: string
  title: string
  messageCount: number
  createdAt: string
  updatedAt: string
}

export interface ChatSessionList {
  items: ChatSessionItem[]
  total: number
  page: number
  pageSize: number
}

export interface ChatSessionMessage {
  id: number
  role: Role
  content: string
  createdAt: string
}

/** 会话明细：messages 按 id 升序；hasMore 表示还有更早的（用首条 id 作 beforeId 继续拉） */
export interface ChatSessionMessages {
  sessionId: string
  messages: ChatSessionMessage[]
  hasMore: boolean
}

/**
 * 预设问答条目：匿名用户在对话页看到的内容。
 * 不调用模型、不消耗 token（`/api/chat` 已改为强制登录，见 docs/04-API接口文档.md §2.5）。
 */
export interface PresetQAItem {
  id: number
  sceneKey: string
  question: string
  answer: string
  sort: number
}

export interface PresetQAList {
  items: PresetQAItem[]
}

export interface ApiEnvelope<T> {
  code: number
  message: string
  data: T
}

export interface HealthData {
  status: string
  version: string
  time: string
}
