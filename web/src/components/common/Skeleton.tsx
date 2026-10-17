import clsx from 'clsx'
import type { CSSProperties } from 'react'

type Props = {
  className?: string
  style?: CSSProperties
}

/**
 * 骨架块（2026-10-17 · UI 优化第 1 轮）。
 *
 * 为什么要有它：此前所有"加载中"都是**一行文字**（`加载中…`）——
 * 界面会先空一片、再"跳"出内容，观感像卡住 ✗。
 * 骨架屏把**即将出现的形状**先画出来，同样的等待会被感知为更短。
 *
 * ⚠️ 只做占位：`aria-hidden` + 真实文案交给旁边（或 `aria-live`）播报，
 * 不要让屏读把骨架当内容读一遍。
 */
export default function Skeleton({ className, style }: Props) {
  return (
    <div aria-hidden style={style} className={clsx('animate-pulse rounded-lg bg-teyvat-fill-2', className)} />
  )
}
