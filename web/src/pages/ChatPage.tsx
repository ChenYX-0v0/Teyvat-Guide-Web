import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { MessagesSquare, Plus } from 'lucide-react'
import MessageBubble from '@/components/chat/MessageBubble'
import ChatInput from '@/components/chat/ChatInput'
import PresetQaPanel from '@/components/chat/PresetQaPanel'
import SessionList from '@/components/chat/SessionList'
import Alert from '@/components/common/Alert'
import EmptyState from '@/components/common/EmptyState'
import { useChatStore } from '@/store/useChatStore'
import { useAuthStore } from '@/store/useAuthStore'
import { genshinApi, type PlayerContextStatus } from '@/api/genshin'
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
    stop,
    loadEarlier,
    startNewSession,
    restore,
  } = useChatStore()
  // 登录态决定视图：未登录只能看预设问答（匿名已禁用 AI 对话，见 docs/04 §2）
  const token = useAuthStore((s) => s.token)
  const bottomRef = useRef<HTMLDivElement>(null)
  // 移动端：历史会话抽屉（桌面端由常驻侧栏承担，不需要这个状态）
  const [sessionsOpen, setSessionsOpen] = useState(false)
  /**
   * 「派蒙能不能看到我的角色」（2026-10-12 新增 ✓，`GET /api/genshin/context-status`）。
   *
   * 用户报障"聊天页取不到绑定 UID 的角色信息"时，排查发现**三条不同原因**
   * （没绑定 / 上游查询失败 / 展柜为空 ✓）对用户是**同一个结果**，且界面上**毫无提示** ✗。
   * ⇒ 进页面时问一次，把原因和下一步说清楚 ✓。
   * ⚠️ 服务端只读本地 ✓（不查 Enka ✗）⇒ 便宜；请求失败则静默（少一行提示，不打扰对话 ✓）。
   */
  const [ctxStatus, setCtxStatus] = useState<PlayerContextStatus | null>(null)

  // 恢复上次会话（仅登录用户）+ 会话列表；匿名则拉预设问答
  useEffect(() => {
    void restore()
  }, [restore, token])

  useEffect(() => {
    if (!token) {
      setCtxStatus(null)
      return
    }
    let alive = true
    genshinApi
      .contextStatus()
      .then((s) => {
        if (alive) setCtxStatus(s)
      })
      .catch(() => {
        if (alive) setCtxStatus(null)
      })
    return () => {
      alive = false
    }
  }, [token])

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
          <h2 className="truncate text-[22px] leading-tight font-semibold tracking-[0.01em] text-teyvat-text">
            {t('chat.title')}
          </h2>
        </div>
        <PresetQaPanel />
      </div>
    )
  }

  return (
    <div className="flex h-full">
      <SessionList />

      <div className="flex h-full min-w-0 flex-1 flex-col">
        <div className="flex h-14 shrink-0 items-center justify-between gap-2 border-b border-teyvat-line px-4 sm:px-6">
          <div className="min-w-0">
            <h2 className="truncate text-[22px] leading-tight font-semibold tracking-[0.01em] text-teyvat-text">
              {t('chat.title')}
            </h2>
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

        {/* 「派蒙看不到你的角色」提示条（2026-10-12）：只在**确实看不到**时出现 ✓，
            并给出可操作的下一步（去账号页绑定 / 说明展柜为空 ✓）—— 替代此前的静默 ✗ */}
        {ctxStatus && !ctxStatus.ready && (
          <div className="flex flex-wrap items-center gap-x-1 border-b border-teyvat-line bg-teyvat-gold/[0.07] px-4 py-2 text-xs text-teyvat-text-2 sm:px-6">
            <span>
              {ctxStatus.reason === 'no_profile' ? t('chat.ctxNoProfile') : t('chat.ctxNotBound')}
            </span>
            <Link
              to="/account"
              className="text-teyvat-gold underline-offset-2 transition-colors hover:underline"
            >
              {t('chat.ctxAction')}
            </Link>
          </div>
        )}

        {/* aria-live：流式回答是"异步更新"，屏读用户必须能听到增量（体检面板早已有，主对话页此前漏了） */}
        <div
          aria-live="polite"
          aria-atomic="false"
          className="flex-1 space-y-5 overflow-y-auto px-4 pb-4 sm:px-6 sm:pb-6"
        >
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
            <EmptyState
              icon={MessagesSquare}
              title={t('chat.emptyHint')}
              className="tx-enter pt-10"
              action={samplers.map((key) => (
                <button
                  key={key}
                  type="button"
                  onClick={() => void sendMessage(t(key))}
                  disabled={loading}
                  className="rounded-full border border-teyvat-line bg-teyvat-fill px-3.5 py-1.5 text-xs text-teyvat-text-2 transition tx-ease tx-fast hover:border-teyvat-line-strong hover:bg-teyvat-gold/5 hover:text-teyvat-gold focus-visible:shadow-glow focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-40"
                >
                  {t(key)}
                </button>
              ))}
            />
          )}
          {messages.map((m, i) => (
            <MessageBubble key={m.id} message={m} caret={loading && i === lastAssistantIdx} />
          ))}
          {error && !hasBubbleError && (
            <Alert tone="error" className="mx-auto max-w-md">
              {error}
            </Alert>
          )}
          <div ref={bottomRef} />
        </div>

        {/* 生成中把发送键换成「停止生成」：掐断流但保留已吐出的文字（见 utils/abort.ts） */}
        <ChatInput busy={loading} onStop={stop} onSend={sendMessage} />
      </div>

      {/* 移动端历史会话抽屉（复用同一个 SessionList，选中后自动关闭） */}
      {sessionsOpen && (
        <div
          className="fixed inset-0 z-40 touch-none md:hidden"
          role="dialog"
          aria-modal="true"
          aria-label={t('chat.sessions')}
        >
          <div
            className="absolute inset-0 bg-teyvat-scrim backdrop-blur-sm"
            onClick={() => setSessionsOpen(false)}
          />
          <div className="absolute inset-y-0 left-0">
            <SessionList variant="drawer" onNavigate={() => setSessionsOpen(false)} />
          </div>
        </div>
      )}
    </div>
  )
}
