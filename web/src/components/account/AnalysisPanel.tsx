import { useEffect, useRef } from 'react'
import clsx from 'clsx'
import { RotateCcw, X } from 'lucide-react'
import MessageBubble from '@/components/chat/MessageBubble'
import ChatInput from '@/components/chat/ChatInput'
import { useAnalysisChat } from '@/hooks/useAnalysisChat'
import { elementColor } from '@/utils/elementColor'
import { t } from '@/i18n'
import type { CharacterCard } from '@/types/genshin'

interface Props {
  open: boolean
  onClose: () => void
  /** 已绑定 UID（空 = 未绑定） */
  uid: string
  /** 当前选中角色（跟随账号页的角色选择条变化） */
  character?: CharacterCard
  /** 展柜角色（用于区分"未绑定"与"未公开"两种空态） */
  characters: CharacterCard[]
}

/**
 * 养成度体检面板：移动端底部抽屉 / 桌面右下浮窗。
 *
 * 两个实现要点（都不是随手写的）：
 *  1. **始终挂载、关闭时只 `hidden`**：关掉面板不等于中断分析——流会照常跑完并落库/记账，
 *     否则用户一关就前功尽弃、下次打开还会重复消耗额度；
 *  2. 面板**不在 Header 之内**（账号页在 `<main>` 里，无 backdrop-filter/transform 祖先），
 *     否则 `fixed` 会被"关"进 Header（该坑见 02-前端开发规范 §5.1）。
 */
export default function AnalysisPanel({ open, onClose, uid, character, characters }: Props) {
  const { messages, loading, restoring, error, dailyLimited, send, rerun } = useAnalysisChat({
    enabled: open,
    uid,
    character,
  })

  const panelRef = useRef<HTMLElement>(null)
  const listRef = useRef<HTMLDivElement>(null)

  // Esc 关闭 + 打开期间锁背景滚动（与 MobileNav 同一套做法）
  useEffect(() => {
    if (!open) return
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKeyDown)
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    // 焦点移到面板本身而不是输入框：手机上打开就弹软键盘会挡住刚要读的报告
    panelRef.current?.focus()
    return () => {
      window.removeEventListener('keydown', onKeyDown)
      document.body.style.overflow = previousOverflow
    }
  }, [open, onClose])

  // 新内容自动滚到底（用 scrollTop 而不是 scrollIntoView，避免把整页也带着滚）
  useEffect(() => {
    const el = listRef.current
    if (el && open) el.scrollTop = el.scrollHeight
  }, [messages, open])

  const blocked = !uid ? t('analysis.needBinding') : characters.length === 0 ? t('account.emptyShowcase') : ''
  const accent = character ? elementColor(character.element) : undefined
  const statusText = restoring
    ? t('analysis.restoring')
    : loading && character
      ? t('analysis.running', { name: character.name })
      : character
        ? t('analysis.reportTitle', { name: character.name })
        : t('analysis.title')

  return (
    <div
      className={clsx(
        'fixed inset-x-0 bottom-0 z-50 h-[85dvh] sm:inset-auto sm:right-6 sm:bottom-6 sm:h-[600px] sm:w-[400px]',
        !open && 'hidden',
      )}
    >
      {/* 移动端遮罩；桌面是浮动窗口，不锁页面 */}
      <div
        className="absolute inset-x-0 bottom-0 h-[100dvh] bg-black/60 sm:hidden"
        onClick={onClose}
        aria-hidden="true"
      />

      <section
        ref={panelRef}
        tabIndex={-1}
        role="dialog"
        aria-modal="true"
        aria-label={t('analysis.title')}
        className="relative flex h-full flex-col overflow-hidden rounded-t-2xl border border-teyvat-line bg-teyvat-panel-soft shadow-panel focus-visible:outline-none sm:rounded-2xl"
      >
        {/* 头部：分析对象 + 重新分析 + 关闭 */}
        <header className="flex items-center gap-3 border-b border-teyvat-line px-4 py-3">
          {character?.iconUrl ? (
            <img
              src={character.iconUrl}
              alt={character.name}
              className="h-9 w-9 shrink-0 rounded-lg border border-teyvat-line-strong object-cover"
            />
          ) : (
            <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border border-teyvat-line-strong bg-teyvat-gold/10 text-sm font-semibold text-teyvat-gold">
              {character?.name[0] ?? '?'}
            </span>
          )}

          <div className="min-w-0 flex-1">
            <p className="flex items-center gap-2 text-sm font-semibold text-teyvat-text">
              <span className="truncate">{character?.name ?? t('analysis.title')}</span>
              {character && (
                <span className="shrink-0 text-[11px] font-normal text-teyvat-text-3">
                  Lv.{character.level}
                </span>
              )}
              {accent && (
                <span
                  className="h-2.5 w-2.5 shrink-0 rounded-full"
                  style={{ background: accent }}
                  title={character?.element}
                />
              )}
            </p>
            <p className="truncate text-[11px] text-teyvat-text-4">{statusText}</p>
          </div>

          {character && (
            <button
              type="button"
              onClick={rerun}
              disabled={loading}
              title={t('analysis.rerun')}
              aria-label={t('analysis.rerun')}
              className="rounded-lg p-1.5 text-teyvat-text-3 transition duration-150 hover:text-teyvat-gold focus-visible:shadow-glow focus-visible:outline-none disabled:opacity-40"
            >
              <RotateCcw className={clsx('h-3.5 w-3.5', loading && 'animate-spin')} />
            </button>
          )}
          <button
            type="button"
            onClick={onClose}
            title={t('header.close')}
            aria-label={t('header.close')}
            className="rounded-lg p-1.5 text-teyvat-text-3 transition duration-150 hover:bg-white/5 hover:text-teyvat-text focus-visible:shadow-glow focus-visible:outline-none"
          >
            <X className="h-4 w-4" />
          </button>
        </header>

        {/* 消息区 */}
        <div ref={listRef} aria-live="polite" className="min-h-0 flex-1 space-y-4 overflow-y-auto px-4 py-4">
          {blocked ? (
            <p className="pt-8 text-center text-sm text-teyvat-text-3">{blocked}</p>
          ) : restoring ? (
            <p className="pt-8 text-center text-sm text-teyvat-text-4">{t('analysis.restoring')}</p>
          ) : messages.length === 0 && !loading ? (
            <p className="pt-8 text-center text-sm text-teyvat-text-4">{t('analysis.empty')}</p>
          ) : (
            messages.map((m, i) => (
              <MessageBubble
                key={m.id}
                message={m}
                caret={loading && i === messages.length - 1 && m.role === 'assistant'}
              />
            ))
          )}

          {error && (
            <div className="flex items-center justify-between gap-3 rounded-xl border border-err/30 bg-err/10 px-3 py-2 text-xs text-err">
              <span className="min-w-0 break-words">{error}</span>
              {character && (
                <button
                  type="button"
                  onClick={rerun}
                  disabled={loading}
                  className="shrink-0 rounded-lg border border-err/40 px-2 py-1 transition duration-150 hover:bg-err/10 focus-visible:shadow-glow focus-visible:outline-none disabled:opacity-40"
                >
                  {t('analysis.retry')}
                </button>
              )}
            </div>
          )}

          {!blocked && (
            <p className="pt-1 text-center text-[11px] leading-snug text-teyvat-text-4">
              {dailyLimited ? t('analysis.dailyLimit') : t('analysis.footnote')}
            </p>
          )}
        </div>

        <ChatInput disabled={Boolean(blocked) || loading} onSend={(text) => void send(text)} />
      </section>
    </div>
  )
}
