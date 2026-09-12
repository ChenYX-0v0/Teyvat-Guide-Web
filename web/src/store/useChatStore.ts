import { create } from 'zustand'
import { chatApi } from '@/api/chat'
import { t } from '@/i18n'
import type { ChatMessage } from '@/types'

interface ChatState {
  sessionId?: string
  messages: ChatMessage[]
  loading: boolean
  error?: string
  sendMessage: (content: string) => Promise<void>
  clear: () => void
}

let seq = 0
const nextId = () => `${Date.now()}-${seq++}`

export const useChatStore = create<ChatState>((set, get) => ({
  sessionId: undefined,
  messages: [],
  loading: false,
  error: undefined,

  async sendMessage(content) {
    const text = content.trim()
    if (!text || get().loading) return

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

    /** 把增量文本追加到正在生成的这条消息上，并结束"等待中"状态 */
    const appendDelta = (delta: string) => {
      set((s) => ({
        messages: s.messages.map((m) =>
          m.id === pendingMsg.id ? { ...m, content: m.content + delta, pending: false } : m,
        ),
      }))
    }

    try {
      await chatApi.stream(
        { sessionId: get().sessionId, message: text },
        {
          onSession: (sessionId) => set({ sessionId }),
          onDelta: appendDelta,
        },
      )

      set((s) => ({
        loading: false,
        messages: s.messages.map((m) =>
          m.id === pendingMsg.id ? { ...m, pending: false, createdAt: Date.now() } : m,
        ),
      }))
    } catch (err) {
      // 中断时已生成的内容保留在气泡里，错误文案另起一行提示
      const message = err instanceof Error ? err.message : t('errors.paimonDistracted')
      set((s) => ({
        loading: false,
        error: message,
        messages: s.messages.map((m) =>
          m.id === pendingMsg.id ? { ...m, pending: false, error: message } : m,
        ),
      }))
    }
  },

  clear() {
    set({ messages: [], sessionId: undefined, error: undefined })
  },
}))
