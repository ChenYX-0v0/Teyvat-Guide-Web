import { useEffect, useRef } from 'react'
import { Plus } from 'lucide-react'
import MessageBubble from '@/components/chat/MessageBubble'
import ChatInput from '@/components/chat/ChatInput'
import SessionList from '@/components/chat/SessionList'
import { useChatStore } from '@/store/useChatStore'
import { t } from '@/i18n'

export default function ChatPage() {
  const {
    messages,
    loading,
    error,
    hasEarlier,
    sendMessage,
    loadEarlier,
    startNewSession,
    restore,
  } = useChatStore()
  const bottomRef = useRef<HTMLDivElement>(null)

  // 恢复上次会话 + 拉会话列表；匿名用户 restore 内部直接返回（匿名不持久化）
  useEffect(() => {
    void restore()
  }, [restore])

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  // 气泡自带的错误提示优先级更高，避免同一条失败既在气泡又在顶部重复展示
  const hasBubbleError = messages.some((m) => m.error)

  return (
    <div className="flex h-full">
      <SessionList />

      <div className="flex h-full min-w-0 flex-1 flex-col">
        <div className="flex items-center justify-between px-6 py-3">
          <div>
            <h2 className="text-base font-medium text-white">{t('chat.title')}</h2>
            <p className="text-xs text-white/40">{t('chat.subtitle')}</p>
          </div>
          <button
            onClick={startNewSession}
            className="flex items-center gap-1.5 rounded-lg border border-white/10 px-3 py-1.5 text-xs text-white/60 transition hover:border-teyvat-gold/40 hover:text-teyvat-gold"
          >
            <Plus className="h-3.5 w-3.5" />
            {t('chat.new')}
          </button>
        </div>

        <div className="flex-1 space-y-5 overflow-y-auto px-6 pb-6">
          {hasEarlier && (
            <div className="pt-4 text-center">
              <button
                onClick={() => void loadEarlier()}
                disabled={loading}
                className="rounded-lg border border-white/10 px-3 py-1.5 text-xs text-white/50 transition hover:text-white/80 disabled:cursor-not-allowed disabled:opacity-40"
              >
                {t('chat.loadEarlier')}
              </button>
            </div>
          )}

          {messages.length === 0 && (
            <div className="mt-20 text-center text-sm text-white/35">
              {t('chat.emptyHint')}
            </div>
          )}
          {messages.map((m) => (
            <MessageBubble key={m.id} message={m} />
          ))}
          {error && !hasBubbleError && (
            <p className="text-center text-xs text-amber-300/80">{error}</p>
          )}
          <div ref={bottomRef} />
        </div>

        <ChatInput disabled={loading} onSend={sendMessage} />
      </div>
    </div>
  )
}
