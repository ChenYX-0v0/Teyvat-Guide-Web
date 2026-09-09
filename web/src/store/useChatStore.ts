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

    try {
      const res = await chatApi.send({
        sessionId: get().sessionId,
        message: text,
      })
      set((s) => ({
        sessionId: res.sessionId,
        loading: false,
        messages: s.messages.map((m) =>
          m.id === pendingMsg.id
            ? { ...m, content: res.reply, pending: false, createdAt: Date.now() }
            : m,
        ),
      }))
    } catch (err) {
      set((s) => ({
        loading: false,
        error: err instanceof Error ? err.message : t('errors.unknown'),
        messages: s.messages.map((m) =>
          m.id === pendingMsg.id
            ? { ...m, pending: false, error: t('errors.paimonDistracted') }
            : m,
        ),
      }))
    }
  },

  clear() {
    set({ messages: [], sessionId: undefined, error: undefined })
  },
}))
