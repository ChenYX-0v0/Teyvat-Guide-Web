import clsx from 'clsx'
import type { HTMLAttributes } from 'react'

type Tone = 'neutral' | 'gold' | 'r5' | 'r4' | 'r3'

type Props = HTMLAttributes<HTMLSpanElement> & {
  /** 左侧小圆点颜色（传 CSS 颜色值，如元素色 token） */
  dot?: string
  tone?: Tone
}

const tones: Record<Tone, string> = {
  neutral: 'border-teyvat-line bg-white/[0.03] text-teyvat-text-2',
  gold: 'border-teyvat-line-strong bg-teyvat-gold/10 text-teyvat-gold-hi',
  // 稀有度沿用原神约定：5★ 金 / 4★ 紫 / 3★ 蓝
  r5: 'border-rarity-5/55 bg-rarity-5/10 text-teyvat-gold-hi',
  r4: 'border-rarity-4/50 bg-rarity-4/10 text-rarity-4',
  r3: 'border-rarity-3/45 bg-rarity-3/10 text-rarity-3',
}

/** 小标签：元素徽记、稀有度、普通标记都用它（元素色只做小面积） */
export default function Chip({ dot, tone = 'neutral', className, children, ...rest }: Props) {
  return (
    <span
      className={clsx(
        'inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs',
        tones[tone],
        className,
      )}
      {...rest}
    >
      {dot && <span className="h-1.5 w-1.5 shrink-0 rounded-full" style={{ background: dot }} />}
      {children}
    </span>
  )
}
