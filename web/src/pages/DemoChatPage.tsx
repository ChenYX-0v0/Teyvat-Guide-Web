import { useCallback, useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { Lock, MessagesSquare, RotateCcw } from 'lucide-react'
import MessageBubble from '@/components/chat/MessageBubble'
import { chatApi } from '@/api/chat'
import { useDemoReplay } from '@/hooks/useDemoReplay'
import { buttonClass } from '@/utils/buttonStyles'
import { EVENTS, track } from '@/lib/track'
import { t } from '@/i18n'
import type { ChatMessage, PresetQAItem } from '@/types'

/**
 * 演示对话（2026-10-17，公开路由 `/demo/chat`；2026-10-17 二次改版对齐真实页，见 TODO-32 ✓）。
 *
 * 与登录后 `ChatPage` 的关系 ✓：**同一个布局骨架** ——
 * 左侧会话列（同宽同位）、标题栏（标题 + 副标题 + 右侧按钮）、提示条、消息区、底部输入区，位置与样式一致 ✓。
 *
 * 刻意保留的不同（都是"功能不可用"的明示 ✓，不是随手不一样 ✗）：
 *  1. 左列没有会话列表，只有一句"演示模式不保存会话"（匿名没有历史会话可言 ✓）；
 *  2. 输入框是**禁用态**并写明要登录，而不是假装能打字 ✗（打完才吃 401 更糟 ✗）；
 *  3. 顶部多一条演示提示条（合规：让访客知道回答是预置示例 ✓）。
 *
 * 回答从哪来 ✓：库里的预设问答（`preset_qas` 的 guest_demo 组，走**已有的免登录**接口
 * `GET /api/preset-qa` ✓），点一条后由 `useDemoReplay` **本地**"思考 + 逐字"演出来 ——
 * 不调模型、不消耗 token、不查 Enka、不落库会话 ✓。
 */
const DEMO_SCENE = 'guest_demo'

export default function DemoChatPage() {
  const [presets, setPresets] = useState<PresetQAItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [messages, setMessages] = useState<ChatMessage[]>([])
  // 正在输出的那条气泡（只有它显示金色呼吸光标；思考阶段为 null —— 气泡自己在转圈）
  const [activeId, setActiveId] = useState<string | null>(null)
  const { phase, play, stop, busy } = useDemoReplay()
  const bottomRef = useRef<HTMLDivElement>(null)

  // 预设问答：**免登录**接口（演示组见 `scripts/gen-demo-mock.mjs` 的刷新口径 ✓）
  useEffect(() => {
    let alive = true
    chatApi
      .presetQa()
      .then((data) => {
        if (!alive) return
        // 只取演示组 ✓；线上若还没配这一组，退回"全部启用项"，别给个空页 ✗
        const demoItems = data.items.filter((i) => i.sceneKey === DEMO_SCENE)
        setPresets(demoItems.length > 0 ? demoItems : data.items)
      })
      .catch((e: unknown) => {
        if (alive) setError(e instanceof Error ? e.message : t('errors.unknown'))
      })
      .finally(() => {
        if (alive) setLoading(false)
      })
    return () => {
      alive = false
    }
  }, [])

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  /** 问一条预设问题：本地"思考 + 逐字回放"，全程零请求 */
  const ask = useCallback(
    (item: PresetQAItem, index: number) => {
      const stamp = Date.now()
      const answerId = `demo-a-${stamp}`
      setActiveId(answerId)
      setMessages((prev) => [
        ...prev,
        { id: `demo-q-${stamp}`, role: 'user', content: item.question, createdAt: stamp },
        { id: answerId, role: 'assistant', content: '', createdAt: stamp, pending: true },
      ])
      // 埋点沿用"点开了第几条预设问答"那一条 ✓（同一个语义，不新增事件名 —— 服务端有白名单）
      track(EVENTS.chatPresetExpanded, { index })
      play(item.answer, {
        onSlice: (text) =>
          setMessages((prev) =>
            prev.map((m) => (m.id === answerId ? { ...m, pending: false, content: text } : m)),
          ),
        onDone: () => setActiveId(null),
      })
    },
    [play],
  )

  const reset = useCallback(() => {
    stop()
    setActiveId(null)
    setMessages([])
  }, [stop])

  return (
    <div className="flex h-full">
      {/* 左列：真实页是历史会话（SessionList），演示页同宽同位给一句说明 ✓ */}
      <aside className="hidden w-56 shrink-0 flex-col border-r border-teyvat-line bg-black/20 p-3 md:flex">
        <div className="flex items-center gap-2 px-2 py-2 text-xs font-medium text-teyvat-text-3">
          <MessagesSquare className="h-3.5 w-3.5" />
          {t('chat.sessions')}
        </div>
        <p className="mt-2 px-2 text-[11px] leading-relaxed text-teyvat-text-4">
          {t('demo.sessionHint')}
        </p>
      </aside>

      <div className="flex h-full min-w-0 flex-1 flex-col">
        {/* 标题栏：与真实页同构（标题 + 副标题 + 右侧动作） */}
        <div className="flex items-center justify-between gap-2 border-b border-teyvat-line px-4 py-3 sm:px-6">
          <div className="min-w-0">
            <h2 className="truncate text-base font-medium text-teyvat-text">{t('chat.title')}</h2>
            <p className="hidden truncate text-xs text-teyvat-text-3 sm:block">{t('chat.subtitle')}</p>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            {messages.length > 0 && (
              <button
                type="button"
                onClick={reset}
                className={buttonClass({ variant: 'ghost', size: 'sm' })}
              >
                <RotateCcw className="h-3.5 w-3.5" />
                {t('demo.chatReset')}
              </button>
            )}
          </div>
        </div>

        {/* 演示提示条：与真实页"派蒙看不到你的角色"提示条同一套样式 ✓ */}
        <div className="flex flex-wrap items-center gap-x-1 border-b border-teyvat-line bg-teyvat-gold/[0.07] px-4 py-2 text-xs text-teyvat-text-2 sm:px-6">
          <span>{t('demo.chatHint')}</span>
        </div>

        <div className="flex-1 space-y-5 overflow-y-auto px-4 pb-4 sm:px-6 sm:pb-6">
          {loading && (
            <p className="py-8 text-center text-sm text-teyvat-text-3">{t('common.loading')}</p>
          )}
          {!loading && error && <p className="py-8 text-center text-sm text-err">{error}</p>}
          {!loading && !error && presets.length === 0 && (
            <p className="py-8 text-center text-sm text-teyvat-text-3">{t('chat.presetEmpty')}</p>
          )}

          {/* 空态：与真实页的示例问题同一个位置与形态（mt-16 居中 + 胶囊 chip ✓） */}
          {messages.length === 0 && presets.length > 0 && (
            <div className="mt-16 text-center">
              <p className="text-sm text-teyvat-text-3">{t('demo.chatPick')}</p>
              <div className="mt-4 flex flex-wrap justify-center gap-2">
                {presets.map((item, index) => (
                  <button
                    key={item.id}
                    type="button"
                    onClick={() => ask(item, index)}
                    disabled={busy}
                    className="rounded-full border border-teyvat-line bg-white/[0.03] px-3 py-1.5 text-xs text-teyvat-text-2 transition duration-150 hover:border-teyvat-line-strong hover:bg-teyvat-gold/5 hover:text-teyvat-gold focus-visible:shadow-glow focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-40"
                  >
                    {item.question}
                  </button>
                ))}
              </div>
            </div>
          )}

          {messages.map((m) => (
            <MessageBubble
              key={m.id}
              message={m}
              caret={m.id === activeId && phase === 'typing'}
            />
          ))}
          <div ref={bottomRef} />
        </div>

        {/* 底部输入区：位置与真实页 ChatInput 一致，但**禁用**并只留登录入口 ✓ */}
        <form
          onSubmit={(e) => e.preventDefault()}
          className="flex items-end gap-2 border-t border-teyvat-line bg-black/25 p-4 pb-[max(1rem,env(safe-area-inset-bottom))] backdrop-blur"
        >
          <div className="flex min-h-11 min-w-0 flex-1 items-center gap-2 rounded-xl border border-teyvat-line bg-white/[0.04] px-4 text-base text-teyvat-text-4 sm:text-sm">
            <Lock className="h-3.5 w-3.5 shrink-0" />
            <span className="truncate">{t('demo.chatLocked')}</span>
          </div>
          <Link
            to="/login"
            className={buttonClass({ variant: 'primary' }) + ' h-11 shrink-0 px-4'}
          >
            {t('demo.loginCta')}
          </Link>
        </form>
      </div>
    </div>
  )
}
