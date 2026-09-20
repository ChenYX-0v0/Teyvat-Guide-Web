import { useState, type FormEvent, type KeyboardEvent } from 'react'
import { Send } from 'lucide-react'
import { t } from '@/i18n'

interface Props {
  disabled?: boolean
  onSend: (text: string) => void
}

export default function ChatInput({ disabled, onSend }: Props) {
  const [value, setValue] = useState('')

  const submit = () => {
    if (!value.trim() || disabled) return
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
      <button
        type="submit"
        disabled={disabled || !value.trim()}
        aria-label={t('chat.send')}
        title={t('chat.send')}
        className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-teyvat-gold text-teyvat-ink transition duration-150 hover:bg-teyvat-gold-hi hover:shadow-glow focus-visible:shadow-glow focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-40"
      >
        <Send className="h-4 w-4" />
      </button>
    </form>
  )
}
