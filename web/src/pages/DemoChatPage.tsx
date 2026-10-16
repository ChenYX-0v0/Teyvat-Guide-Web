import { useCallback, useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { Lock, RotateCcw, Sparkles } from 'lucide-react'
import MessageBubble from '@/components/chat/MessageBubble'
import { chatApi } from '@/api/chat'
import { buttonClass } from '@/utils/buttonStyles'
import { EVENTS, track } from '@/lib/track'
import { t } from '@/i18n'
import type { ChatMessage, PresetQAItem } from '@/types'

/**
 * 演示对话（2026-10-17，公开路由 `/demo/chat`）。
 *
 * 与登录后的 `ChatPage` 的区别只有一点：**弹出来的回答是本地回放的** ✓。
 * 问题来自库里的预设问答（`preset_qas` 的 guest_demo 组，6 条 ✓，走**已有的**免登录接口 `GET /api/preset-qa` ✓），
 * 回答本来就在库里 —— 这里先用"思考"点几下、再逐字吐出来，让访客看到派蒙作答的节奏 ✓。
 *
 * 为什么坚持本地回放而不是"匿名也能调模型" ✗：
 *   - 匿名调模型=任何人都能白嫖额度，且没有账号可追责（口径见 shared-docs/05 §1）；
 *   - 演示要的是"看清楚产品长什么样"，预置答案足够 ✓，且**不消耗 token、不查 Enka、不落库会话** ✓。
 */
const THINK_MS = 900 // "思考"（气泡三个呼吸点）的时长
const TYPE_TICK_MS = 24 // 逐字输出的节拍
const CHARS_PER_TICK = 2 // 每拍吐几个字（中文 2 字/24ms ≈ 每秒 80 字，接近真人打字速度）

/**
 * 演示组的分组键（`preset_qas.scene_key`）。
 *
 * ⚠️ 必须只取这一组 ✗：线上 `preset_qas` 里**同时**躺着老分组（onboarding / build / explore，给匿名对话页兜底用 ✓），
 * 全列出来演示页会混进 6 条和演示无关的通用问答 ✗（2026-10-17 自检发现线上 12 条里只有 6 条是演示组 ✓）。
 * 这一组也正是 `scripts/gen-demo-mock.mjs qa` 刷新的对象 ✓ —— 两边认的是同一个键 ✓。
 */
const DEMO_SCENE = 'guest_demo'

/** 回放阶段：idle 可点 / thinking 思考中 / typing 逐字输出中 */
type Phase = 'idle' | 'thinking' | 'typing'

export default function DemoChatPage() {
  const [presets, setPresets] = useState<PresetQAItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [phase, setPhase] = useState<Phase>('idle')
  // 正在输出的那条气泡（只有它显示金色呼吸光标）；思考阶段为 null（此时气泡自己在转圈）
  const [activeId, setActiveId] = useState<string | null>(null)
  // 计时器放 ref：连点问题时先掐掉上一段回放，否则两次输出会叠在同一条气泡上 ✗
  const replay = useRef<{ think?: number; type?: number }>({})
  const bottomRef = useRef<HTMLDivElement>(null)

  /** 停掉当前回放（幂等：没在回放时调用无副作用） */
  const stop = useCallback(() => {
    if (replay.current.think) window.clearTimeout(replay.current.think)
    if (replay.current.type) window.clearInterval(replay.current.type)
    replay.current = {}
    setPhase('idle')
    setActiveId(null)
  }, [])

  // 预设问答：**免登录**接口（匿名用户在对话页看到的就是这一组内容）
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

  // 卸载时清计时器：留着会在已卸载的组件上 setState
  useEffect(() => stop, [stop])

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  /** 问一条预设问题：本地"思考 + 逐字回放"，全程零请求 */
  const ask = useCallback(
    (item: PresetQAItem, index: number) => {
      stop()
      const stamp = Date.now()
      const answerId = `demo-a-${stamp}`
      setMessages((prev) => [
        ...prev,
        { id: `demo-q-${stamp}`, role: 'user', content: item.question, createdAt: stamp },
        { id: answerId, role: 'assistant', content: '', createdAt: stamp, pending: true },
      ])
      setPhase('thinking')
      setActiveId(answerId)
      // 埋点沿用"点开了第几条预设问答"那一条 ✓（同一个语义，不新增事件名 —— 服务端有白名单）
      track(EVENTS.chatPresetExpanded, { index })

      replay.current.think = window.setTimeout(() => {
        replay.current.think = undefined
        setPhase('typing')
        let printed = 0
        replay.current.type = window.setInterval(() => {
          printed = Math.min(item.answer.length, printed + CHARS_PER_TICK)
          const slice = item.answer.slice(0, printed)
          setMessages((prev) =>
            prev.map((m) => (m.id === answerId ? { ...m, pending: false, content: slice } : m)),
          )
          if (printed >= item.answer.length) {
            if (replay.current.type) window.clearInterval(replay.current.type)
            replay.current = {}
            setPhase('idle')
            setActiveId(null)
          }
        }, TYPE_TICK_MS)
      }, THINK_MS)
    },
    [stop],
  )

  const reset = useCallback(() => {
    stop()
    setMessages([])
  }, [stop])

  const busy = phase !== 'idle'

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="shrink-0 border-b border-teyvat-line px-4 py-3 sm:px-6">
        <h2 className="flex items-center gap-2 text-sm font-medium text-teyvat-text">
          <Sparkles className="h-4 w-4 text-teyvat-gold" />
          {t('demo.chatTitle')}
        </h2>
        <p className="mt-1 text-[11px] leading-relaxed text-teyvat-text-4">{t('demo.chatHint')}</p>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto px-4 py-4 sm:px-6">
        {loading && (
          <p className="py-8 text-center text-sm text-teyvat-text-3">{t('common.loading')}</p>
        )}
        {!loading && error && <p className="py-8 text-center text-sm text-err">{error}</p>}
        {!loading && !error && presets.length === 0 && (
          <p className="py-8 text-center text-sm text-teyvat-text-3">{t('chat.presetEmpty')}</p>
        )}

        {presets.length > 0 && (
          <div className="mb-5">
            <p className="mb-2 text-[11px] text-teyvat-text-4">{t('demo.chatPick')}</p>
            <div className="flex flex-wrap gap-2">
              {presets.map((item, index) => (
                <button
                  key={item.id}
                  type="button"
                  onClick={() => ask(item, index)}
                  disabled={busy}
                  className="rounded-full border border-teyvat-line bg-white/[0.03] px-3 py-1.5 text-left text-xs text-teyvat-text-2 transition duration-150 hover:border-teyvat-line-strong hover:bg-teyvat-gold/5 hover:text-teyvat-gold focus-visible:shadow-glow focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-40"
                >
                  {item.question}
                </button>
              ))}
            </div>
          </div>
        )}

        {messages.length > 0 && (
          <div className="space-y-5">
            {messages.map((m) => (
              <MessageBubble key={m.id} message={m} caret={m.id === activeId && phase === 'typing'} />
            ))}
            <div ref={bottomRef} />
          </div>
        )}
      </div>

      {/* 底部输入区：演示不可输入（不假装能用 —— 打完字才吃 401 更糟 ✗），只做登录引导 */}
      <div className="shrink-0 border-t border-teyvat-line bg-black/25 p-4 backdrop-blur">
        <div className="flex items-center gap-3">
          <div className="flex min-h-11 min-w-0 flex-1 items-center gap-2 rounded-xl border border-teyvat-line bg-white/[0.04] px-4 text-base text-teyvat-text-4 sm:text-sm">
            <Lock className="h-3.5 w-3.5 shrink-0" />
            <span className="truncate">{t('demo.chatLocked')}</span>
          </div>
          {messages.length > 0 && (
            <button type="button" onClick={reset} className={buttonClass({ variant: 'ghost' })}>
              <RotateCcw className="h-3.5 w-3.5" />
              {t('demo.chatReset')}
            </button>
          )}
          <Link to="/login" className={buttonClass({ variant: 'primary' })}>
            {t('demo.loginCta')}
          </Link>
        </div>
      </div>
    </div>
  )
}
