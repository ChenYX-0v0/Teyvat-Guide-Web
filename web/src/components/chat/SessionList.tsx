import { useState } from 'react'
import { Check, MessageSquarePlus, Pencil, Trash2, X } from 'lucide-react'
import clsx from 'clsx'
import Skeleton from '@/components/common/Skeleton'
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
      ? 'hidden w-64 shrink-0 flex-col border-r border-teyvat-line bg-teyvat-sunken md:flex'
      : 'flex h-full w-72 max-w-[85vw] flex-col border-r border-teyvat-line bg-teyvat-panel-soft'

  return (
    <aside className={rootClass}>
      <div className="flex h-14 shrink-0 items-center justify-between px-3">
        <span className="text-[11px] font-medium tracking-[0.08em] text-teyvat-text-4">
          {t('chat.sessions')}
        </span>
        {token && (
          <button
            onClick={() => {
              startNewSession()
              onNavigate?.()
            }}
            title={t('chat.new')}
            className="flex h-7 w-7 tap-44 items-center justify-center rounded-lg text-teyvat-text-3 transition tx-ease tx-fast hover:bg-teyvat-fill-2 hover:text-teyvat-gold focus-visible:shadow-glow focus-visible:outline-none"
          >
            <MessageSquarePlus className="h-4 w-4" />
          </button>
        )}
      </div>

      {!token ? (
        <p className="px-4 py-4 text-xs leading-relaxed text-teyvat-text-4">{t('chat.loginHint')}</p>
      ) : (
        <div className="flex-1 space-y-1 overflow-y-auto px-2 pb-3">
          {/* 加载中：画 4 行骨架（形状先到位，避免"空一片再跳出内容"） */}
          {sessionsLoading && sessions.length === 0 && (
            <div className="space-y-2 px-1 py-1" aria-hidden>
              {[64, 88, 52, 76].map((w) => (
                <Skeleton key={w} className="h-7" style={{ width: `${w}%` }} />
              ))}
            </div>
          )}
          {!sessionsLoading && sessions.length === 0 && (
            <p className="px-2 py-3 text-xs leading-relaxed text-teyvat-text-4">
              {t('chat.sessionsEmpty')}
            </p>
          )}

          {sessions.map((item) => {
            const active = item.sessionId === sessionId
            return (
              <div
                key={item.sessionId}
                className={clsx(
                  'group flex items-center gap-1 rounded-xl px-2 py-2 transition tx-ease tx-fast',
                  active
                    ? 'bg-teyvat-raised font-medium text-teyvat-text'
                    : 'text-teyvat-text-2 hover:bg-teyvat-fill-2 hover:text-teyvat-text',
                )}
              >
                {confirmingId === item.sessionId ? (
                  <>
                    <span className="min-w-0 flex-1 truncate text-xs text-err">
                      {t('chat.deleteConfirm')}
                    </span>
                    <button
                      onClick={() => void submitDelete(item.sessionId)}
                      title={t('chat.delete')}
                      className="flex h-7 w-7 shrink-0 items-center justify-center rounded text-err transition hover:text-err/80"
                    >
                      <Check className="h-3.5 w-3.5" />
                    </button>
                    <button
                      onClick={() => setConfirmingId(undefined)}
                      title={t('chat.renameCancel')}
                      className="flex h-7 w-7 shrink-0 items-center justify-center rounded text-teyvat-text-3 transition hover:text-teyvat-text"
                    >
                      <X className="h-3.5 w-3.5" />
                    </button>
                  </>
                ) : editingId === item.sessionId ? (
                  <>
                    <input
                      name="sessionTitle"
                      aria-label={t('chat.rename')}
                      autoFocus
                      value={draft}
                      maxLength={50}
                      onChange={(e) => setDraft(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter') void submitRename()
                        if (e.key === 'Escape') cancelRename()
                      }}
                      className="min-w-0 flex-1 rounded border border-teyvat-line-strong bg-teyvat-fill px-1.5 py-1 text-xs text-teyvat-text outline-none focus:shadow-glow"
                    />
                    <button
                      onClick={() => void submitRename()}
                      title={t('chat.renameSave')}
                      className="flex h-7 w-7 shrink-0 items-center justify-center rounded text-teyvat-text-3 transition hover:text-teyvat-gold"
                    >
                      <Check className="h-3.5 w-3.5" />
                    </button>
                    <button
                      onClick={cancelRename}
                      title={t('chat.renameCancel')}
                      className="flex h-7 w-7 shrink-0 items-center justify-center rounded text-teyvat-text-3 transition hover:text-err"
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
                      className="min-w-0 flex-1 truncate text-left text-xs focus-visible:text-teyvat-gold focus-visible:outline-none"
                    >
                      {item.title || t('chat.untitled')}
                    </button>
                    <button
                      onClick={() => beginRename(item.sessionId, item.title)}
                      title={t('chat.rename')}
                      className="hidden h-7 w-7 shrink-0 items-center justify-center rounded text-teyvat-text-4 transition group-hover:flex hover:text-teyvat-gold"
                    >
                      <Pencil className="h-3 w-3" />
                    </button>
                    <button
                      onClick={() => beginDelete(item.sessionId)}
                      title={t('chat.delete')}
                      className="hidden h-7 w-7 shrink-0 items-center justify-center rounded text-teyvat-text-4 transition group-hover:flex hover:text-err"
                    >
                      <Trash2 className="h-3 w-3" />
                    </button>
                  </>
                )}
              </div>
            )
          })}

          {error && <p className="px-2 py-1 text-xs text-warn/90">{error}</p>}
        </div>
      )}
    </aside>
  )
}
