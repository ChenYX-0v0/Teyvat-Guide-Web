import type { ReactNode } from 'react'
import type { LucideIcon } from 'lucide-react'
import clsx from 'clsx'

type Props = {
  /** 金色线条图标（不给就只显示文字，保持极简） */
  icon?: LucideIcon
  /** 一句主文案（说明"现在没有内容"） */
  title: ReactNode
  /** 一句补充（说明"接下来该做什么"），可选 */
  hint?: ReactNode
  /** 行动区（按钮 / 示例胶囊），可选 */
  action?: ReactNode
  className?: string
}

/**
 * 空态（2026-10-17 · UI 优化第 1 轮）。
 *
 * 此前空态是"一行灰字 + 若干胶囊"，散落各页、各写各的 ✗。
 * 统一成：**图标徽记 + 一句主文案 + 一句下一步 + 一个行动区**。
 * 空态不是"没东西"，而是"告诉用户下一步该点哪里" —— 这是转化点，不是占位符。
 */
export default function EmptyState({ icon: Icon, title, hint, action, className }: Props) {
  return (
    <div className={clsx('flex flex-col items-center justify-center px-6 py-12 text-center', className)}>
      {Icon ? (
        <span className="mb-4 flex h-12 w-12 items-center justify-center rounded-xl border border-teyvat-line-strong bg-teyvat-gold/10 text-teyvat-gold">
          <Icon className="h-5 w-5" />
        </span>
      ) : null}
      <p className="text-sm font-medium text-teyvat-text-2">{title}</p>
      {hint ? (
        <p className="mt-1.5 max-w-sm text-xs leading-relaxed text-teyvat-text-4">{hint}</p>
      ) : null}
      {action ? <div className="mt-5 flex flex-wrap justify-center gap-2">{action}</div> : null}
    </div>
  )
}
