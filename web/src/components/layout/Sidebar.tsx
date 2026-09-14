import { NavLink } from 'react-router-dom'
import clsx from 'clsx'
import { navItems } from './navItems'

/** 桌面端常驻侧栏（<md 不渲染，移动端用 MobileNav 抽屉） */
export default function Sidebar() {
  return (
    <aside className="hidden w-56 shrink-0 border-r border-white/10 bg-black/20 p-3 md:block">
      <nav className="flex flex-col gap-1">
        {navItems.map(({ to, label, icon: Icon, end }) => (
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
