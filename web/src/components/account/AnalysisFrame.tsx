import { useEffect, useRef, type ReactNode } from 'react'
import clsx from 'clsx'
import { RotateCcw, X } from 'lucide-react'
import { elementColor } from '@/utils/elementColor'
import { t } from '@/i18n'
import type { CharacterCard } from '@/types/genshin'

interface Props {
  open: boolean
  onClose: () => void
  /** 分析对象（头部头像 / 名字 / 等级 / 元素色） */
  character?: CharacterCard
  /** 头部第二行：状态文案（分析中 / 报告标题 / 演示说明 ✓） */
  subtitle: string
  /** 重新生成：**不传就不显示按钮** ✓（演示页是本地回放，单独给「重新播放」） */
  onRerun?: () => void
  rerunning?: boolean
  /** 内容变化信号：值一变就自动滚到底（真实页传 messages、演示页传文本 ✓） */
  scrollSignal?: unknown
  /** 底部固定区（真实页是 `ChatInput`，演示页是登录引导 ✓） */
  footer: ReactNode
  children: ReactNode
}

/**
 * 体检面板的**纯外壳**（2026-10-17 从 `AnalysisPanel` 抽出，见 TODO-32 ✓）：
 * 定位、遮罩、头部、滚动区、Esc / 锁滚动、打开后的焦点处理都在这里 ✓。
 *
 * 抽出来的唯一理由 ✓：演示页（`/demo`）要让"派蒙的分析"以**同一个面板形态**出现 ✓
 * （此前是页内一张卡 ✗，与真实页的右下浮窗完全不同）；两处共用一份外壳，避免各写一套后漂移 ✗。
 *
 * 三个实现要点（都是从原组件搬过来的，不是随口定的）：
 *  1. **始终挂载、关闭时只 `hidden`**：关掉面板不等于中断——真实页的流照常跑完并落库/记账 ✓；
 *  2. 面板**不在 Header 之内**（页面在 `<main>` 里，无 backdrop-filter/transform 祖先 ✓），
 *     否则 `fixed` 会被"关"进 Header（该坑见 02-前端开发规范 §5.1）；
 *  3. 打开时焦点给**面板本身**而不是输入框：手机上打开就弹软键盘会挡住刚要读的报告 ✓。
 */
export default function AnalysisFrame({
  open,
  onClose,
  character,
  subtitle,
  onRerun,
  rerunning = false,
  scrollSignal,
  footer,
  children,
}: Props) {
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
  }, [scrollSignal, open])

  const accent = character ? elementColor(character.element) : undefined

  return (
    <div
      className={clsx(
        'fixed inset-x-0 bottom-0 z-50 h-[85dvh] sm:inset-auto sm:right-6 sm:bottom-6 sm:h-[600px] sm:w-[400px]',
        !open && 'hidden',
      )}
    >
      {/* 移动端遮罩；桌面是浮动窗口，不锁页面 */}
      <div
        className="absolute inset-x-0 bottom-0 h-[100dvh] bg-teyvat-scrim sm:hidden"
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
              width={36}
              height={36}
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
            <p className="truncate text-[11px] text-teyvat-text-4">{subtitle}</p>
          </div>

          {onRerun && (
            <button
              type="button"
              onClick={onRerun}
              disabled={rerunning}
              title={t('analysis.rerun')}
              aria-label={t('analysis.rerun')}
              className="tap-44 rounded-lg p-1.5 text-teyvat-text-3 transition tx-ease tx-fast hover:text-teyvat-gold focus-visible:shadow-glow focus-visible:outline-none disabled:opacity-40"
            >
              {/* 旋转加在 wrapper 上：直接转 <svg> 会带来 transform-origin / 重绘问题 */}
              <span className={clsx('inline-flex', rerunning && 'animate-spin')}>
                <RotateCcw className="h-3.5 w-3.5" />
              </span>
            </button>
          )}
          <button
            type="button"
            onClick={onClose}
            title={t('header.close')}
            aria-label={t('header.close')}
            className="tap-44 rounded-lg p-1.5 text-teyvat-text-3 transition tx-ease tx-fast hover:bg-teyvat-fill-2 hover:text-teyvat-text focus-visible:shadow-glow focus-visible:outline-none"
          >
            <X className="h-4 w-4" />
          </button>
        </header>

        {/* 消息区 */}
        <div
          ref={listRef}
          aria-live="polite"
          className="min-h-0 flex-1 space-y-4 overflow-y-auto overscroll-contain px-4 py-4"
        >
          {children}
        </div>

        {footer}
      </section>
    </div>
  )
}
