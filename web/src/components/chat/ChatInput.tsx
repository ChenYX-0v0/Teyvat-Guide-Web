import { useState, type FormEvent, type KeyboardEvent } from 'react'
import { Send, Square } from 'lucide-react'
import { t } from '@/i18n'

interface Props {
  disabled?: boolean
  /** 正在生成：发送键换成「停止生成」（只掐断流，不续写；见 utils/abort.ts） */
  busy?: boolean
  onSend: (text: string) => void
  /** 不传则只禁用发送、不显示停止键（面板没接这个能力时保持原样） */
  onStop?: () => void
}

export default function ChatInput({ disabled, busy, onSend, onStop }: Props) {
  const [value, setValue] = useState('')
  const canStop = Boolean(busy && onStop)

  const submit = () => {
    // 生成中回车也不许发：这一轮还没收尾（要发就先按停止）
    if (!value.trim() || disabled || busy) return
    onSend(value)
    setValue('')
  }

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    submit()
  }

  const onKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      submit()
    }
  }

  // 输入框字号在移动端 ≥16px（iOS 聚焦更小字号会自动放大页面），sm 起回到 text-sm
  return (
    <form
      onSubmit={onSubmit}
      className="flex items-end gap-2 border-t border-teyvat-line bg-black/25 p-4 pb-[max(1rem,env(safe-area-inset-bottom))] backdrop-blur"
    >
      <textarea
        value={value}
        onChange={(e) => setValue(e.target.value)}
        onKeyDown={onKeyDown}
        rows={1}
        placeholder={t('chat.inputPlaceholder')}
        className="max-h-40 min-h-11 flex-1 resize-none rounded-xl border border-teyvat-line bg-white/[0.04] px-4 py-2.5 text-base text-teyvat-text outline-none transition placeholder:text-teyvat-text-4 focus:border-teyvat-line-strong focus:shadow-glow sm:text-sm"
      />
      {canStop ? (
        <button
          type="button"
          onClick={onStop}
          aria-label={t('chat.stop')}
          title={t('chat.stop')}
          className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl border border-teyvat-line-strong bg-teyvat-raised text-teyvat-text-2 transition duration-150 hover:border-warn/60 hover:text-warn focus-visible:shadow-glow focus-visible:outline-none"
        >
          <Square className="h-3.5 w-3.5 fill-current" />
        </button>
      ) : (
        <button
          type="submit"
          disabled={disabled || !value.trim()}
          aria-label={t('chat.send')}
          title={t('chat.send')}
          className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-teyvat-gold text-teyvat-ink transition duration-150 hover:bg-teyvat-gold-hi hover:shadow-glow focus-visible:shadow-glow focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-40"
        >
          <Send className="h-4 w-4" />
        </button>
      )}
    </form>
  )
}
