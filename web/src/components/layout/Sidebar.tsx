import { NavLink } from 'react-router-dom'
import { House, MessagesSquare, BookOpen, UserRound, Shield } from 'lucide-react'
import clsx from 'clsx'
import { t } from '@/i18n'
import { useAuthStore } from '@/store/useAuthStore'

export default function Sidebar() {
  const isAdmin = useAuthStore((s) => s.role === 'admin')

  const items = [
    { to: '/', label: t('sidebar.home'), icon: House, end: true },
    { to: '/chat', label: t('sidebar.chat'), icon: MessagesSquare, end: false },
    { to: '/account', label: t('sidebar.account'), icon: UserRound, end: false },
    { to: '/wiki', label: t('sidebar.wiki'), icon: BookOpen, end: false },
    // 管理员入口：仅管理员角色可见
    ...(isAdmin
      ? [{ to: '/admin', label: t('sidebar.admin'), icon: Shield, end: false }]
      : []),
  ]

  return (
    <aside className="hidden w-56 shrink-0 border-r border-white/10 bg-black/20 p-3 md:block">
      <nav className="flex flex-col gap-1">
        {items.map(({ to, label, icon: Icon, end }) => (
          <NavLink
            key={to}
            to={to}
            end={end}
            className={({ isActive }) =>
              clsx(
                'flex items-center gap-2 rounded-lg px-3 py-2 text-sm transition',
                isActive
                  ? 'bg-teyvat-gold/15 text-teyvat-gold'
                  : 'text-white/65 hover:bg-white/5 hover:text-white',
              )
            }
          >
            <Icon className="h-4 w-4" />
            {label}
          </NavLink>
        ))}
      </nav>
    </aside>
  )
}
