import clsx from 'clsx'

export type ButtonVariant = 'primary' | 'soft' | 'ghost'
export type ButtonSize = 'sm' | 'md'

const base =
  'inline-flex items-center justify-center whitespace-nowrap transition tx-ease tx-fast focus-visible:shadow-glow focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-40'

// 尺寸用**固定高度**（而不是 padding 撑）：同一行的按钮高度一致，视觉上才"齐"
const sizes: Record<ButtonSize, string> = {
  sm: 'h-8 gap-1.5 rounded-lg px-3 text-xs',
  md: 'h-10 gap-2 rounded-xl px-4 text-sm',
}

const variants: Record<ButtonVariant, string> = {
  // 金色只给"主操作"；悬停时给发光，其余状态保持克制
  primary:
    'bg-teyvat-gold font-semibold text-teyvat-ink shadow-panel hover:bg-teyvat-gold-hi hover:shadow-glow',
  soft: 'border border-teyvat-line bg-teyvat-raised text-teyvat-text hover:border-teyvat-line-strong hover:bg-teyvat-fill-2',
  ghost:
    'border border-teyvat-line text-teyvat-text-2 hover:border-teyvat-line-strong hover:bg-teyvat-gold/5 hover:text-teyvat-gold',
}

/**
 * 按钮样式表。
 * 单独放 `utils/` 的原因：`Button.tsx` 若同时导出组件与函数会触发
 * react-refresh 的 only-export-components 警告；而且这里需要被 `<Link>` 复用
 * （`<a>` 里不能塞 `<button>`）。
 */
export function buttonClass(opts: { variant?: ButtonVariant; size?: ButtonSize } = {}) {
  const { variant = 'ghost', size = 'md' } = opts
  return clsx(base, sizes[size], variants[variant])
}
