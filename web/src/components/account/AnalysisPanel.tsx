import MessageBubble from '@/components/chat/MessageBubble'
import ChatInput from '@/components/chat/ChatInput'
import AnalysisFrame from './AnalysisFrame'
import { useAnalysisChat } from '@/hooks/useAnalysisChat'
import { t } from '@/i18n'
import type { CharacterCard } from '@/types/genshin'

interface Props {
  open: boolean
  onClose: () => void
  /** 已绑定 UID（空 = 未绑定） */
  uid: string
  /** 当前选中角色（跟随账号页的角色选择条变化） */
  character?: CharacterCard
  /** 展柜角色（用于区分"未绑定"与"未公开"两种空态） */
  characters: CharacterCard[]
}

/**
 * 养成度体检面板：移动端底部抽屉 / 桌面右下浮窗。
 *
 * 2026-10-17：**外壳搬到 `AnalysisFrame`** ✓（定位、遮罩、头部、Esc / 锁滚动 / 焦点、自动滚到底），
 * 这里只剩"内容与流式逻辑" ✓ —— 这样演示页的分析面板能复用**同一个外壳** ✓，
 * 而不是另写一张页内卡 ✗（见 TODO-32）。
 */
export default function AnalysisPanel({ open, onClose, uid, character, characters }: Props) {
  const { messages, loading, restoring, error, dailyLimited, send, rerun, stop } = useAnalysisChat({
    enabled: open,
    uid,
    character,
  })

  const blocked = !uid ? t('analysis.needBinding') : characters.length === 0 ? t('account.emptyShowcase') : ''
  const statusText = restoring
    ? t('analysis.restoring')
    : loading && character
      ? t('analysis.running', { name: character.name })
      : character
        ? t('analysis.reportTitle', { name: character.name })
        : t('analysis.title')

  return (
    <AnalysisFrame
      open={open}
      onClose={onClose}
      character={character}
      subtitle={statusText}
      // 没有角色就没得重跑（与原实现一致：按钮只在有 character 时出现 ✓）
      onRerun={character ? rerun : undefined}
      rerunning={loading}
      scrollSignal={messages}
      footer={
        <ChatInput
          disabled={Boolean(blocked)}
          busy={loading}
          onStop={stop}
          onSend={(text) => void send(text)}
        />
      }
    >
      {blocked ? (
        <p className="pt-8 text-center text-sm text-teyvat-text-3">{blocked}</p>
      ) : restoring ? (
        <p className="pt-8 text-center text-sm text-teyvat-text-4">{t('analysis.restoring')}</p>
      ) : messages.length === 0 && !loading ? (
        <p className="pt-8 text-center text-sm text-teyvat-text-4">{t('analysis.empty')}</p>
      ) : (
        messages.map((m, i) => (
          <MessageBubble
            key={m.id}
            message={m}
            caret={loading && i === messages.length - 1 && m.role === 'assistant'}
          />
        ))
      )}

      {error && (
        <div className="flex items-center justify-between gap-3 rounded-xl border border-err/30 bg-err/10 px-3 py-2 text-xs text-err">
          <span className="min-w-0 break-words">{error}</span>
          {character && (
            <button
              type="button"
              onClick={rerun}
              disabled={loading}
              className="shrink-0 rounded-lg border border-err/40 px-2 py-1 transition duration-150 hover:bg-err/10 focus-visible:shadow-glow focus-visible:outline-none disabled:opacity-40"
            >
              {t('analysis.retry')}
            </button>
          )}
        </div>
      )}

      {!blocked && (
        <p className="pt-1 text-center text-[11px] leading-snug text-teyvat-text-4">
          {dailyLimited ? t('analysis.dailyLimit') : t('analysis.footnote')}
        </p>
      )}
    </AnalysisFrame>
  )
}
