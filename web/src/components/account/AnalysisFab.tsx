import { useEffect, useRef, useState } from 'react'
import AnalysisPanel from './AnalysisPanel'
import AnalysisFabButton from './AnalysisFabButton'
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
 *
 * 2026-10-17：按钮视觉抽到 `AnalysisFabButton` ✓（演示页复用同一颗 ✓），
 * 这里只留逻辑：置灰判断 + 埋点 + 关闭后焦点归还 ✓。
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
        <AnalysisFabButton
          buttonRef={fabRef}
          disabled={disabled}
          label={label}
          onClick={() => {
            if (disabled) return
            setOpen(true)
            track(EVENTS.accountAnalysisOpen)
          }}
        />
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
