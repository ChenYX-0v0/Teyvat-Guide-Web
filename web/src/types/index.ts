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
