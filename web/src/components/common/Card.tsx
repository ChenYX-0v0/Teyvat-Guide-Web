import clsx from 'clsx'
import type { HTMLAttributes } from 'react'

type Props = HTMLAttributes<HTMLDivElement> & {
  /** panel = L1 常规卡片（默认）｜soft = L2 浮层内的卡 */
  variant?: 'panel' | 'soft'
  /** 悬停微抬升（可点击的卡片用） */
  lift?: boolean
}

/**
 * 基础卡片：统一"材质"——顶缘高光 + 投影（`shadow-panel`）。
 * 这是视觉方案里"物件感"的来源，别在页面里手写 `border border-white/10 bg-...` 了。
 */
export default function Card({ variant = 'panel', lift, className, ...rest }: Props) {
  return (
    <div
      className={clsx(
        'rounded-2xl border border-teyvat-line shadow-panel',
        variant === 'panel' ? 'bg-teyvat-panel' : 'bg-teyvat-panel-soft',
        lift && 'transition duration-150 hover:-translate-y-0.5 hover:border-teyvat-line-strong',
        className,
      )}
      {...rest}
    />
  )
}
