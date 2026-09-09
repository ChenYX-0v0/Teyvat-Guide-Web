import { Bot, User } from 'lucide-react'
import clsx from 'clsx'
import type { ChatMessage } from '@/types'

interface Props {
  message: ChatMessage
}

export default function MessageBubble({ message }: Props) {
  const isUser = message.role === 'user'

  return (
    <div className={clsx('flex gap-3', isUser ? 'flex-row-reverse' : 'flex-row')}>
      <div
        className={clsx(
          'mt-1 flex h-8 w-8 shrink-0 items-center justify-center rounded-full',
          isUser ? 'bg-white/10' : 'bg-teyvat-gold/15 text-teyvat-gold',
        )}
      >
        {isUser ? <User className="h-4 w-4" /> : <Bot className="h-4 w-4" />}
      </div>
      <div
        className={clsx(
          'max-w-[75%] rounded-2xl px-4 py-2.5 text-sm leading-relaxed whitespace-pre-wrap break-words',
          isUser
            ? 'bg-teyvat-gold/15 text-white'
            : 'border border-white/10 bg-teyvat-panel/70 text-white/85',
        )}
      >
        {message.pending ? (
          <span className="flex gap-1 py-1">
            {[0, 150, 300].map((delay) => (
              <span
                key={delay}
                className="h-1.5 w-1.5 animate-bounce rounded-full bg-teyvat-gold"
                style={{ animationDelay: `${delay}ms` }}
              />
            ))}
          </span>
        ) : (
          message.content || message.error
        )}
      </div>
    </div>
  )
}
