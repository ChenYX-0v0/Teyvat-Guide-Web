import type { ReactNode } from 'react'
import clsx from 'clsx'

export type AlertTone = 'info' | 'warn' | 'error' | 'ok'

const tones: Record<AlertTone, string> = {
  // 提示：金调（与"强调"一致，不抢错误态的注意力）
  info: 'border-teyvat-line-strong bg-teyvat-gold/5 text-teyvat-text-2',
  warn: 'border-warn/30 bg-warn/10 text-warn',
  error: 'border-err/30 bg-err/10 text-err',
  ok: 'border-ok/30 bg-ok/10 text-ok',
}

type Props = {
  tone?: AlertTone
  children: ReactNode
  /** 右侧操作（重试 / 去处理 / 关闭） */
  action?: ReactNode
  className?: string
}

/**
 * 内联提示条（2026-10-17 · UI 优化第 1 轮）。
 *
 * 以前每个页面各写一段 `<div className="rounded-2xl border ...">` ✗：
 * 成功是绿色块、错误是红色块，字号 13/14 混用、圆角 12/16 混用 —— 同一件事长得不一样。
 * 现在统一：**1 行 12px + 圆角 12 + 左侧文案 + 右侧操作**，四种语气。
 */
export default function Alert({ tone = 'info', children, action, className }: Props) {
  return (
    <div
      role={tone === 'error' ? 'alert' : 'status'}
      className={clsx(
        'flex items-start gap-3 rounded-xl border px-3 py-2 text-xs leading-relaxed',
        tones[tone],
        className,
      )}
    >
      <div className="min-w-0 flex-1 break-words">{children}</div>
      {action ? <div className="shrink-0">{action}</div> : null}
    </div>
  )
}
