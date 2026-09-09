import { useEffect, useRef } from 'react'
import { Trash2 } from 'lucide-react'
import MessageBubble from '@/components/chat/MessageBubble'
import ChatInput from '@/components/chat/ChatInput'
import { useChatStore } from '@/store/useChatStore'
import { t } from '@/i18n'

export default function ChatPage() {
  const { messages, loading, sendMessage, clear } = useChatStore()
  const bottomRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  return (
    <div className="flex h-full flex-col">
      <div className="flex items-center justify-between px-6 py-3">
        <div>
          <h2 className="text-base font-medium text-white">{t('chat.title')}</h2>
          <p className="text-xs text-white/40">{t('chat.subtitle')}</p>
        </div>
        <button
          onClick={clear}
          className="flex items-center gap-1.5 rounded-lg border border-white/10 px-3 py-1.5 text-xs text-white/60 transition hover:border-red-400/40 hover:text-red-300"
        >
          <Trash2 className="h-3.5 w-3.5" />
          {t('chat.clear')}
        </button>
      </div>

      <div className="flex-1 space-y-5 overflow-y-auto px-6 pb-6">
        {messages.length === 0 && (
          <div className="mt-20 text-center text-sm text-white/35">
            {t('chat.emptyHint')}
          </div>
        )}
        {messages.map((m) => (
          <MessageBubble key={m.id} message={m} />
        ))}
        <div ref={bottomRef} />
      </div>

      <ChatInput disabled={loading} onSend={sendMessage} />
    </div>
  )
}
