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

  return (
    <form
      onSubmit={onSubmit}
      className="flex items-end gap-2 border-t border-white/10 bg-black/25 p-4 backdrop-blur"
    >
      <textarea
        value={value}
        onChange={(e) => setValue(e.target.value)}
        onKeyDown={onKeyDown}
        rows={1}
        placeholder={t('chat.inputPlaceholder')}
        className="max-h-40 min-h-11 flex-1 resize-none rounded-xl border border-white/10 bg-white/5 px-4 py-2.5 text-sm outline-none transition placeholder:text-white/30 focus:border-teyvat-gold/50"
      />
      <button
        type="submit"
        disabled={disabled || !value.trim()}
        className="flex h-11 w-11 items-center justify-center rounded-xl bg-teyvat-gold text-teyvat-ink transition hover:bg-teyvat-gold-deep disabled:cursor-not-allowed disabled:opacity-40"
      >
        <Send className="h-4 w-4" />
      </button>
    </form>
  )
}
