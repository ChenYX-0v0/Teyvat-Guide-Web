import type { Ref } from 'react'
import { Sparkles } from 'lucide-react'

interface Props {
  label: string
  disabled?: boolean
  onClick: () => void
  /** 关闭面板后要把焦点还回来（见 AnalysisFab ✓） */
  buttonRef?: Ref<HTMLButtonElement>
}

/**
 * 右下角那颗金色悬浮按钮（2026-10-17 从 `AnalysisFab` 抽出，见 TODO-32 ✓）。
 *
 * 抽出来是为了让演示页的"看看派蒙的分析"入口**长得一模一样** ✓ ——
 * 演示页不该另做一颗按钮 ✗，否则访客会以为真实页的入口在别处 ✗。
 */
export default function AnalysisFabButton({ label, disabled, onClick, buttonRef }: Props) {
  return (
    <button
      ref={buttonRef}
      type="button"
      onClick={onClick}
      disabled={disabled}
      title={label}
      aria-label={label}
      className="fixed right-4 bottom-[max(1rem,env(safe-area-inset-bottom))] z-40 flex h-14 w-14 items-center justify-center rounded-full border border-teyvat-line-strong bg-gradient-to-b from-teyvat-gold-hi to-teyvat-gold text-teyvat-ink shadow-panel transition duration-150 hover:shadow-glow focus-visible:shadow-glow focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-40 sm:right-6 sm:bottom-6"
    >
      <Sparkles className="h-5 w-5" />
    </button>
  )
}
