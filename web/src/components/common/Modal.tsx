import { useEffect, type ReactNode } from 'react'

interface Props {
  open: boolean
  /**
   * 阻断模式（用于"必须阅读并同意才能继续"）：
   * 不响应 Esc、不响应点遮罩关闭、不渲染任何关闭按钮。
   */
  blocking?: boolean
  title?: ReactNode
  onClose?: () => void
  /** 底部操作区（按钮），固定不随内容滚动 */
  footer?: ReactNode
  children: ReactNode
}

/**
 * 极简弹窗（无第三方依赖）。
 *
 * 为什么自己写而不是引库：全站只用到这一个交互（须知弹窗 / 条款查看），
 * 引一个 dialog 库会带来一套新的可访问性约定与样式体系，得不偿失。
 */
export default function Modal({ open, blocking = false, title, onClose, footer, children }: Props) {
  useEffect(() => {
    if (!open || blocking) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose?.()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [open, blocking, onClose])

  if (!open) return null

  return (
    <div
      role="dialog"
      aria-modal="true"
      className="fixed inset-0 z-50 flex items-center justify-center bg-teyvat-scrim px-3 py-6 backdrop-blur-sm sm:px-4"
      onClick={() => {
        if (!blocking) onClose?.()
      }}
    >
      <div
        className="tx-enter flex max-h-full w-full max-w-2xl flex-col overflow-hidden rounded-2xl border border-teyvat-line-strong bg-teyvat-panel shadow-panel"
        onClick={(e) => e.stopPropagation()}
      >
        {title ? (
          <div className="shrink-0 border-b border-teyvat-line px-5 py-3.5 text-[13px] font-semibold tracking-[0.04em] text-teyvat-gold">
            {title}
          </div>
        ) : null}

        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">{children}</div>

        {footer ? <div className="shrink-0 border-t border-teyvat-line px-5 py-3">{footer}</div> : null}
      </div>
    </div>
  )
}
