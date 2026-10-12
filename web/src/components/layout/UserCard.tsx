import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { ChevronUp, KeyRound, LogOut, MessageSquarePlus, UserRound } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import clsx from 'clsx'
import ProfileModal from '@/components/account/ProfileModal'
import PasswordModal from '@/components/account/PasswordModal'
import FeedbackModal from '@/components/account/FeedbackModal'
import { useAuthStore } from '@/store/useAuthStore'
import { t } from '@/i18n'

type Props = {
  /** 移动端抽屉里用时传 onClose：点了条目顺手把抽屉收起来 ✓ */
  onNavigate?: () => void
}

/**
 * 侧栏/抽屉**底部**的用户卡片（2026-10-11，用户要求）。
 *
 * 形态：一块近似长方形、贴底的卡片 ✓（`mt-auto` 由调用方的列布局压到底部）；
 * 点击后向上弹出四个功能 —— **个人信息管理 / 修改密码 / 功能反馈 / 退出登录** ✓
 * （功能反馈是 2026-10-12 加的 ✓，提交后落库 `feedbacks` 专表，见 `account/FeedbackModal.tsx`）。
 *
 * 为什么用"向上弹的浮层"而不是居中弹窗 ✗：卡片本身就在屏幕底部，
 * 向下弹会被视口裁掉 ✓。前三个功能各自再开一个 `Modal`（表单要空间 ✓），
 * 退出登录直接执行（不值得再弹一次确认 —— 本站在登录页有明确入口，误点成本低 ✓）。
 *
 * 桌面侧栏（`Sidebar`）与移动抽屉（`MobileNav`）**共用这一个组件** ✓，
 * 免得两边各写一份后漂移 ✗（导航项当年就是这么统一的 ✓）。
 */
export default function UserCard({ onNavigate }: Props) {
  const displayName = useAuthStore((s) => s.displayName)
  const logout = useAuthStore((s) => s.logout)
  const navigate = useNavigate()

  const [menuOpen, setMenuOpen] = useState(false)
  const [modal, setModal] = useState<'profile' | 'password' | 'feedback' | null>(null)
  const boxRef = useRef<HTMLDivElement>(null)

  // 点外部 / Esc 关掉浮层（浮层不是 Modal：它没有遮罩，不该锁页面 ✓）
  useEffect(() => {
    if (!menuOpen) return
    const onDown = (e: MouseEvent) => {
      if (boxRef.current && !boxRef.current.contains(e.target as Node)) setMenuOpen(false)
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setMenuOpen(false)
    }
    window.addEventListener('mousedown', onDown)
    window.addEventListener('keydown', onKey)
    return () => {
      window.removeEventListener('mousedown', onDown)
      window.removeEventListener('keydown', onKey)
    }
  }, [menuOpen])

  // 昵称可能是空的（用户清空过 ✓）⇒ 回落展示，别显示成一片空白 ✗
  const name = displayName || t('user.fallbackName')

  const openModal = (which: 'profile' | 'password' | 'feedback') => {
    setMenuOpen(false)
    setModal(which)
  }

  const doLogout = () => {
    setMenuOpen(false)
    logout()
    onNavigate?.()
    navigate('/login', { replace: true })
  }

  return (
    <div ref={boxRef} className="relative mt-auto">
      {menuOpen ? (
        <div className="absolute bottom-full left-0 mb-2 w-full overflow-hidden rounded-xl border border-teyvat-line-strong bg-teyvat-panel shadow-panel">
          <MenuRow icon={UserRound} label={t('user.profile')} onClick={() => openModal('profile')} />
          <MenuRow icon={KeyRound} label={t('user.password')} onClick={() => openModal('password')} />
          <MenuRow
            icon={MessageSquarePlus}
            label={t('user.feedback')}
            onClick={() => openModal('feedback')}
          />
          <div className="h-px bg-teyvat-line" />
          <MenuRow icon={LogOut} label={t('header.logout')} onClick={doLogout} danger />
        </div>
      ) : null}

      <button
        type="button"
        onClick={() => setMenuOpen((v) => !v)}
        aria-expanded={menuOpen}
        aria-label={t('user.menu')}
        className={clsx(
          'flex w-full items-center gap-2 rounded-xl border px-3 py-2 text-left transition focus-visible:shadow-glow focus-visible:outline-none',
          menuOpen
            ? 'border-teyvat-line-strong bg-white/5'
            : 'border-teyvat-line bg-white/[0.03] hover:border-teyvat-line-strong hover:bg-white/5',
        )}
      >
        <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg border border-teyvat-line-strong bg-teyvat-gold/10 text-[11px] font-semibold text-teyvat-gold">
          {name.slice(0, 1).toUpperCase()}
        </span>
        <span className="min-w-0 flex-1 truncate text-xs text-teyvat-text-2">{name}</span>
        <ChevronUp
          className={clsx('h-3.5 w-3.5 shrink-0 text-teyvat-text-4 transition', menuOpen ? '' : 'rotate-180')}
        />
      </button>

      <ProfileModal open={modal === 'profile'} onClose={() => setModal(null)} />
      <PasswordModal open={modal === 'password'} onClose={() => setModal(null)} />
      <FeedbackModal open={modal === 'feedback'} onClose={() => setModal(null)} />
    </div>
  )
}

function MenuRow({
  icon: Icon,
  label,
  onClick,
  danger = false,
}: {
  icon: LucideIcon
  label: string
  onClick: () => void
  danger?: boolean
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={clsx(
        'flex w-full items-center gap-2 px-3 py-2 text-left text-xs transition',
        danger
          ? 'text-teyvat-text-3 hover:bg-err/10 hover:text-err'
          : 'text-teyvat-text-2 hover:bg-white/5 hover:text-teyvat-text',
      )}
    >
      <Icon className="h-3.5 w-3.5" />
      {label}
    </button>
  )
}
