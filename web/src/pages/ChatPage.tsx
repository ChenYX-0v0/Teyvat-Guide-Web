import { useEffect, useRef, useState } from 'react'
import { MessagesSquare, Plus } from 'lucide-react'
import MessageBubble from '@/components/chat/MessageBubble'
import ChatInput from '@/components/chat/ChatInput'
import PresetQaPanel from '@/components/chat/PresetQaPanel'
import SessionList from '@/components/chat/SessionList'
import { useChatStore } from '@/store/useChatStore'
import { useAuthStore } from '@/store/useAuthStore'
import { buttonClass } from '@/utils/buttonStyles'
import { t } from '@/i18n'

/** 空态示例问题（点一下即问，降低"第一句不知道问什么"的门槛） */
const samplers = ['chat.emptySample1', 'chat.emptySample2', 'chat.emptySample3'] as const

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
  // 登录态决定视图：未登录只能看预设问答（匿名已禁用 AI 对话，见 docs/04 §2）
  const token = useAuthStore((s) => s.token)
  const bottomRef = useRef<HTMLDivElement>(null)
  // 移动端：历史会话抽屉（桌面端由常驻侧栏承担，不需要这个状态）
  const [sessionsOpen, setSessionsOpen] = useState(false)

  // 恢复上次会话（仅登录用户）+ 会话列表；匿名则拉预设问答
  useEffect(() => {
    void restore()
  }, [restore, token])

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  // 气泡自带的错误提示优先级更高，避免同一条失败既在气泡又在顶部重复展示
  const hasBubbleError = messages.some((m) => m.error)

  // 只有最后一条助手消息在生成中时显示呼吸光标（纯视觉，不下发任何请求）
  let lastAssistantIdx = -1
  for (let i = messages.length - 1; i >= 0; i -= 1) {
    if (messages[i].role === 'assistant') {
      lastAssistantIdx = i
      break
    }
  }

  // 匿名：预设问答 + 登录引导（没有会话侧栏，也没有可用的输入框）
  if (!token) {
    return (
      <div className="flex h-full min-w-0 flex-1 flex-col">
        <div className="px-4 py-3 sm:px-6">
          <h2 className="truncate text-base font-medium text-teyvat-text">{t('chat.title')}</h2>
          <p className="hidden truncate text-xs text-teyvat-text-3 sm:block">{t('chat.subtitle')}</p>
        </div>
        <PresetQaPanel />
      </div>
    )
  }

  return (
    <div className="flex h-full">
      <SessionList />

      <div className="flex h-full min-w-0 flex-1 flex-col">
        <div className="flex items-center justify-between gap-2 border-b border-teyvat-line px-4 py-3 sm:px-6">
          <div className="min-w-0">
            <h2 className="truncate text-base font-medium text-teyvat-text">{t('chat.title')}</h2>
            <p className="hidden truncate text-xs text-teyvat-text-3 sm:block">
              {t('chat.subtitle')}
            </p>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            <button
              onClick={() => setSessionsOpen(true)}
              className={buttonClass({ variant: 'ghost', size: 'sm' }) + ' md:hidden'}
            >
              <MessagesSquare className="h-3.5 w-3.5" />
              {t('chat.sessions')}
            </button>
            <button onClick={startNewSession} className={buttonClass({ variant: 'ghost', size: 'sm' })}>
              <Plus className="h-3.5 w-3.5" />
              {t('chat.new')}
            </button>
          </div>
        </div>

        <div className="flex-1 space-y-5 overflow-y-auto px-4 pb-4 sm:px-6 sm:pb-6">
          {hasEarlier && (
            <div className="pt-4 text-center">
              <button
                onClick={() => void loadEarlier()}
                disabled={loading}
                className={buttonClass({ variant: 'ghost', size: 'sm' })}
              >
                {t('chat.loadEarlier')}
              </button>
            </div>
          )}

          {messages.length === 0 && (
            <div className="mt-16 text-center">
              <p className="text-sm text-teyvat-text-3">{t('chat.emptyHint')}</p>
              <div className="mt-4 flex flex-wrap justify-center gap-2">
                {samplers.map((key) => (
                  <button
                    key={key}
                    type="button"
                    onClick={() => void sendMessage(t(key))}
                    disabled={loading}
                    className="rounded-full border border-teyvat-line bg-white/[0.03] px-3 py-1.5 text-xs text-teyvat-text-2 transition duration-150 hover:border-teyvat-line-strong hover:bg-teyvat-gold/5 hover:text-teyvat-gold focus-visible:shadow-glow focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-40"
                  >
                    {t(key)}
                  </button>
                ))}
              </div>
            </div>
          )}
          {messages.map((m, i) => (
            <MessageBubble key={m.id} message={m} caret={loading && i === lastAssistantIdx} />
          ))}
          {error && !hasBubbleError && <p className="text-center text-xs text-warn/90">{error}</p>}
          <div ref={bottomRef} />
        </div>

        <ChatInput disabled={loading} onSend={sendMessage} />
      </div>

      {/* 移动端历史会话抽屉（复用同一个 SessionList，选中后自动关闭） */}
      {sessionsOpen && (
        <div className="fixed inset-0 z-40 touch-none md:hidden" role="dialog" aria-modal="true">
          <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" onClick={() => setSessionsOpen(false)} />
          <div className="absolute inset-y-0 left-0">
            <SessionList variant="drawer" onNavigate={() => setSessionsOpen(false)} />
          </div>
        </div>
      )}
    </div>
  )
}
