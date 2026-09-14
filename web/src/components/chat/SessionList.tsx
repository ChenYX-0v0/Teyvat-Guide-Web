import { useState } from 'react'
import { Check, MessageSquarePlus, Pencil, Trash2, X } from 'lucide-react'
import clsx from 'clsx'
import { useChatStore } from '@/store/useChatStore'
import { useAuthStore } from '@/store/useAuthStore'
import { t } from '@/i18n'

type Props = {
  /** sidebar = 桌面常驻侧栏；drawer = 移动端抽屉内容 */
  variant?: 'sidebar' | 'drawer'
  /** 抽屉场景：选中会话 / 新建对话后关闭抽屉 */
  onNavigate?: () => void
}

/**
 * 历史会话列表。
 * 列表只有登录用户才有（匿名不持久化，也没有"我的会话"这个概念），
 * 因此未登录时只给一句登录引导。契约见 docs/04-API接口文档.md §2.4。
 *
 * 删除是软删除：只从自己的列表移除，服务端保留（后台审计仍可查），
 * 因此这里做一次就地二次确认，避免误点。
 *
 * 同一个组件两处复用：桌面是 `hidden md:flex` 的侧栏，移动端放进抽屉
 * （`variant="drawer"`）——避免"手机上看不到历史会话"那类两套实现漂移。
 */
export default function SessionList({ variant = 'sidebar', onNavigate }: Props) {
  const {
    sessions,
    sessionsLoading,
    sessionId,
    openSession,
    startNewSession,
    renameSession,
    deleteSession,
  } = useChatStore()
  const token = useAuthStore((s) => s.token)

  const [editingId, setEditingId] = useState<string>()
  const [draft, setDraft] = useState('')
  const [confirmingId, setConfirmingId] = useState<string>()
  const [error, setError] = useState<string>()

  const beginRename = (id: string, title: string) => {
    setConfirmingId(undefined)
    setEditingId(id)
    setDraft(title)
    setError(undefined)
  }

  const cancelRename = () => {
    setEditingId(undefined)
    setDraft('')
  }

  const submitRename = async () => {
    if (!editingId) return
    const title = draft.trim()
    if (!title) {
      setError(t('chat.renameEmpty'))
      return
    }
    try {
      await renameSession(editingId, title)
      cancelRename()
    } catch {
      setError(t('chat.renameFailed'))
    }
  }

  const beginDelete = (id: string) => {
    setEditingId(undefined)
    setConfirmingId(id)
    setError(undefined)
  }

  const submitDelete = async (id: string) => {
    try {
      await deleteSession(id)
      setConfirmingId(undefined)
    } catch {
      setError(t('chat.deleteFailed'))
    }
  }

  const rootClass =
    variant === 'sidebar'
      ? 'hidden w-64 shrink-0 flex-col border-r border-white/10 bg-black/20 md:flex'
      : 'flex h-full w-72 max-w-[85vw] flex-col border-r border-white/10 bg-teyvat-ink'

  return (
    <aside className={rootClass}>
      <div className="flex items-center justify-between px-3 py-3">
        <span className="text-xs font-medium text-white/50">{t('chat.sessions')}</span>
        {token && (
          <button
            onClick={() => {
              startNewSession()
              onNavigate?.()
            }}
            title={t('chat.new')}
            className="flex h-7 w-7 items-center justify-center rounded-lg text-white/50 transition hover:bg-white/5 hover:text-teyvat-gold"
          >
            <MessageSquarePlus className="h-4 w-4" />
          </button>
        )}
      </div>

      {!token ? (
        <p className="px-4 py-4 text-xs leading-relaxed text-white/35">{t('chat.loginHint')}</p>
      ) : (
        <div className="flex-1 space-y-1 overflow-y-auto px-2 pb-3">
          {sessionsLoading && sessions.length === 0 && (
            <p className="px-2 py-1 text-xs text-white/30">{t('common.loading')}</p>
          )}
          {!sessionsLoading && sessions.length === 0 && (
            <p className="px-2 py-1 text-xs text-white/30">{t('chat.sessionsEmpty')}</p>
          )}

          {sessions.map((item) => {
            const active = item.sessionId === sessionId
            return (
              <div
                key={item.sessionId}
                className={clsx(
                  'group flex items-center gap-1 rounded-lg px-2 py-2 transition',
                  active
                    ? 'bg-teyvat-gold/15 text-teyvat-gold'
                    : 'text-white/65 hover:bg-white/5 hover:text-white',
                )}
              >
                {confirmingId === item.sessionId ? (
                  <>
                    <span className="min-w-0 flex-1 truncate text-xs text-red-300">
                      {t('chat.deleteConfirm')}
                    </span>
                    <button
                      onClick={() => void submitDelete(item.sessionId)}
                      title={t('chat.delete')}
                      className="flex h-6 w-6 shrink-0 items-center justify-center rounded text-red-300 transition hover:text-red-200"
                    >
                      <Check className="h-3.5 w-3.5" />
                    </button>
                    <button
                      onClick={() => setConfirmingId(undefined)}
                      title={t('chat.renameCancel')}
                      className="flex h-6 w-6 shrink-0 items-center justify-center rounded text-white/50 transition hover:text-white"
                    >
                      <X className="h-3.5 w-3.5" />
                    </button>
                  </>
                ) : editingId === item.sessionId ? (
                  <>
                    <input
                      autoFocus
                      value={draft}
                      maxLength={50}
                      onChange={(e) => setDraft(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter') void submitRename()
                        if (e.key === 'Escape') cancelRename()
                      }}
                      className="min-w-0 flex-1 rounded border border-teyvat-gold/40 bg-white/5 px-1.5 py-1 text-xs text-white outline-none"
                    />
                    <button
                      onClick={() => void submitRename()}
                      title={t('chat.renameSave')}
                      className="flex h-6 w-6 shrink-0 items-center justify-center rounded text-white/50 transition hover:text-teyvat-gold"
                    >
                      <Check className="h-3.5 w-3.5" />
                    </button>
                    <button
                      onClick={cancelRename}
                      title={t('chat.renameCancel')}
                      className="flex h-6 w-6 shrink-0 items-center justify-center rounded text-white/50 transition hover:text-red-300"
                    >
                      <X className="h-3.5 w-3.5" />
                    </button>
                  </>
                ) : (
                  <>
                    <button
                      onClick={() => {
                        void openSession(item.sessionId)
                        onNavigate?.()
                      }}
                      title={item.title}
                      className="min-w-0 flex-1 truncate text-left text-xs"
                    >
                      {item.title || t('chat.untitled')}
                    </button>
                    <button
                      onClick={() => beginRename(item.sessionId, item.title)}
                      title={t('chat.rename')}
                      className="hidden h-6 w-6 shrink-0 items-center justify-center rounded text-white/40 transition group-hover:flex hover:text-teyvat-gold"
                    >
                      <Pencil className="h-3 w-3" />
                    </button>
                    <button
                      onClick={() => beginDelete(item.sessionId)}
                      title={t('chat.delete')}
                      className="hidden h-6 w-6 shrink-0 items-center justify-center rounded text-white/40 transition group-hover:flex hover:text-red-300"
                    >
                      <Trash2 className="h-3 w-3" />
                    </button>
                  </>
                )}
              </div>
            )
          })}

          {error && <p className="px-2 py-1 text-xs text-amber-300/80">{error}</p>}
        </div>
      )}
    </aside>
  )
}
