import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import AnalysisFabButton from './AnalysisFabButton'
import AnalysisFrame from './AnalysisFrame'
import MessageBubble from '@/components/chat/MessageBubble'
import { useDemoReplay } from '@/hooks/useDemoReplay'
import { buttonClass } from '@/utils/buttonStyles'
import { t } from '@/i18n'
import type { ChatMessage } from '@/types'
import type { CharacterCard } from '@/types/genshin'

interface Props {
  /** 当前选中角色（跟随下方角色选择条 ✓） */
  character?: CharacterCard
  /** 库里预置的分析（**不是**现场生成的 ✓） */
  analysis: string
  /** 换角色时重新回放（传 charKey ✓） */
  charKey: string
}

/**
 * 演示档案页的"派蒙的分析"（2026-10-17，见 TODO-32 ✓）。
 *
 * 形态与真实账号页**完全一致** ✓：右下角同一颗金色悬浮按钮 ✓（`AnalysisFabButton`）
 * ＋ 同一个浮动面板外壳 ✓（`AnalysisFrame`）—— 此前是页内一张卡 ✗，访客会以为真实页的分析也长在页面里 ✗。
 *
 * 内容差异只有一处（**必须不同** ✓）：这里不调模型，把库里那份预置分析用本地计时器"演"出来 ✓
 * （思考 → 逐字，见 `useDemoReplay`）；面板底部因此也没有输入框，换成登录引导 ✓。
 */
export default function DemoAnalysis({ character, analysis, charKey }: Props) {
  const [open, setOpen] = useState(false)
  const [text, setText] = useState('')
  const { phase, play } = useDemoReplay()

  const replay = useCallback(() => {
    setText('')
    play(analysis, { onSlice: setText })
  }, [play, analysis])

  // 打开面板 / 换角色即回放（纯本地计时器，零请求 ✓）
  useEffect(() => {
    if (open) replay()
  }, [open, replay, charKey])

  const message: ChatMessage = {
    id: `demo-analysis-${charKey}`,
    role: 'assistant',
    content: text,
    createdAt: 0,
    pending: phase === 'thinking',
  }

  return (
    <>
      {!open && <AnalysisFabButton label={t('analysis.fab')} onClick={() => setOpen(true)} />}

      <AnalysisFrame
        open={open}
        onClose={() => setOpen(false)}
        character={character}
        subtitle={t('demo.analysisNote')}
        onRerun={replay}
        rerunning={phase !== 'idle'}
        scrollSignal={text}
        footer={
          <div className="border-t border-teyvat-line bg-teyvat-sunken p-4 pb-[max(1rem,env(safe-area-inset-bottom))] backdrop-blur">
            <Link
              to="/login"
              className={buttonClass({ variant: 'primary' }) + ' w-full justify-center'}
            >
              {t('demo.analysisCta')}
            </Link>
          </div>
        }
      >
        <MessageBubble message={message} caret={phase === 'typing'} />
      </AnalysisFrame>
    </>
  )
}
