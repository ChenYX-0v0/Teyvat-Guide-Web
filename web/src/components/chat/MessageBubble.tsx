import { Bot, User } from 'lucide-react'
import clsx from 'clsx'
import type { ChatMessage } from '@/types'
import { t } from '@/i18n'

interface Props {
  message: ChatMessage
  /** 正在流式生成（只有最后一条助手消息会传 true）：在文末显示金色呼吸光标 */
  caret?: boolean
}

export default function MessageBubble({ message, caret }: Props) {
  const isUser = message.role === 'user'

  return (
    <div className={clsx('flex gap-3', isUser ? 'flex-row-reverse' : 'flex-row')}>
      <div
        className={clsx(
          'mt-1 flex h-8 w-8 shrink-0 items-center justify-center rounded-full border',
          isUser
            ? 'border-teyvat-line bg-teyvat-raised text-teyvat-text-2'
            : 'border-teyvat-line-strong bg-teyvat-gold/15 text-teyvat-gold',
        )}
      >
        {isUser ? <User className="h-4 w-4" /> : <Bot className="h-4 w-4" />}
      </div>
      <div
        className={clsx(
          'max-w-[75%] rounded-2xl px-4 py-2.5 text-sm leading-relaxed whitespace-pre-wrap break-words',
          isUser
            ? 'border border-teyvat-line-strong bg-teyvat-gold/12 text-teyvat-text'
            : // 助手：面板材质 + 左侧金色细条（材质与"派蒙在说话"的视觉锚点）
              'relative border border-teyvat-line bg-teyvat-panel text-teyvat-text shadow-panel before:absolute before:top-3.5 before:bottom-3.5 before:left-[7px] before:w-[2px] before:rounded-full before:bg-gradient-to-b before:from-teyvat-gold-hi before:to-teyvat-gold/15',
        )}
        style={!isUser ? { paddingLeft: '20px' } : undefined}
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
          <>
            {message.content}
            {caret && <span className="caret-breathe" />}
            {message.error && (
              <p className={clsx('text-xs text-warn/90', message.content && 'mt-1.5')}>
                {message.error}
              </p>
            )}
            {/*
              AI 生成内容标识（合规要求，见 teyvat-docs/web/11 §7.4）：
              每个助手回答末尾都有一条，明示"机器生成、仅供参考"。
              用户消息不加；生成中（pending）不加。
            */}
            {!isUser && message.content && (
              <p className="mt-1.5 text-[10px] leading-none text-teyvat-text-4">
                {t('chat.aiGenerated')}
              </p>
            )}
          </>
        )}
      </div>
    </div>
  )
}
