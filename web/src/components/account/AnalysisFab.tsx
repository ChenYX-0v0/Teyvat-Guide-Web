import { useEffect, useRef, useState } from 'react'
import { Sparkles } from 'lucide-react'
import AnalysisPanel from './AnalysisPanel'
import { t } from '@/i18n'
import { EVENTS, track } from '@/lib/track'
import type { CharacterCard } from '@/types/genshin'

interface Props {
  /** 已绑定 UID（空 = 未绑定 → 按钮置灰并说明原因） */
  uid: string
  /** 展柜角色 */
  characters: CharacterCard[]
  /** 当前选中角色（由账号页持有，见设计文档 §7.5） */
  character?: CharacterCard
}

/**
 * 账号页悬浮体检入口：右下角按钮 + 面板（真源：teyvat-docs/web/13）。
 * 未绑定 UID 时**禁用而不是隐藏**——用户需要看到"为什么不能问"。
 */
export default function AnalysisFab({ uid, characters, character }: Props) {
  const [open, setOpen] = useState(false)
  const fabRef = useRef<HTMLButtonElement>(null)
  const disabled = !uid
  const label = disabled ? t('analysis.needBinding') : t('analysis.fab')

  const close = () => setOpen(false)

  // 关闭后焦点回到按钮（按钮是关闭后才重新挂载的，必须等渲染完再 focus）
  const wasOpenRef = useRef(false)
  useEffect(() => {
    if (wasOpenRef.current && !open) fabRef.current?.focus()
    wasOpenRef.current = open
  }, [open])

  return (
    <>
      {!open && (
        <button
          ref={fabRef}
          type="button"
          onClick={() => {
            if (disabled) return
            setOpen(true)
            track(EVENTS.accountAnalysisOpen)
          }}
          disabled={disabled}
          title={label}
          aria-label={label}
          className="fixed right-4 bottom-[max(1rem,env(safe-area-inset-bottom))] z-40 flex h-14 w-14 items-center justify-center rounded-full border border-teyvat-line-strong bg-gradient-to-b from-teyvat-gold-hi to-teyvat-gold text-teyvat-ink shadow-panel transition duration-150 hover:shadow-glow focus-visible:shadow-glow focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-40 sm:right-6 sm:bottom-6"
        >
          <Sparkles className="h-5 w-5" />
        </button>
      )}

      <AnalysisPanel
        open={open}
        onClose={close}
        uid={uid}
        character={character}
        characters={characters}
      />
    </>
  )
}
