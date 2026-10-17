import clsx from 'clsx'
import type { HTMLAttributes } from 'react'

type Props = HTMLAttributes<HTMLDivElement> & {
  /**
   * panel = L1 主卡片（默认，带投影）
   * soft  = L2 浮层内的卡（更亮一档）
   * inset = **内嵌块**：卡片里的小块（无投影、圆角小一档）——
   *         用来制造"套娃但克制"的纵深，而不是给每个小块都套一张完整卡片
   */
  variant?: 'panel' | 'soft' | 'inset'
  /** 悬停微抬升（可点击的卡片用） */
  lift?: boolean
}

const variants: Record<NonNullable<Props['variant']>, string> = {
  panel: 'rounded-2xl border border-teyvat-line bg-teyvat-panel shadow-panel',
  soft: 'rounded-2xl border border-teyvat-line bg-teyvat-panel-soft shadow-panel',
  inset: 'rounded-xl border border-teyvat-line bg-teyvat-fill',
}

/**
 * 基础卡片：统一"材质"——顶缘高光 + 投影（`shadow-panel`）。
 * 别在页面里手写 `border border-white/10 bg-...` 了。
 *
 * 2026-10-17：新增 `inset` 一档。此前**所有**容器都是同一种"圆角 16 + 投影"，
 * 卡片里的小块也套一张完整卡片 ⇒ 层级没有主次、界面显平 ✗。
 */
export default function Card({ variant = 'panel', lift, className, ...rest }: Props) {
  return (
    <div
      className={clsx(
        variants[variant],
        lift &&
          'transition tx-ease tx-fast hover:-translate-y-0.5 hover:border-teyvat-line-strong',
        className,
      )}
      {...rest}
    />
  )
}
