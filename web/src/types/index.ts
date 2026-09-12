export type Role = 'user' | 'assistant' | 'system'

export interface ChatMessage {
  id: string
  role: Role
  content: string
  createdAt: number
  pending?: boolean
  error?: string
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
