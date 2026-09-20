import { NavLink } from 'react-router-dom'
import clsx from 'clsx'
import { navItems } from './navItems'

/** 桌面端常驻侧栏（<md 不渲染，移动端用 MobileNav 抽屉） */
export default function Sidebar() {
  return (
    <aside className="hidden w-56 shrink-0 border-r border-teyvat-line bg-black/20 p-3 md:block">
      <nav className="flex flex-col gap-1">
        {navItems.map(({ to, label, icon: Icon, end }) => (
          <NavLink
            key={to}
            to={to}
            end={end}
            className={({ isActive }) =>
              clsx(
                'relative flex items-center gap-2 rounded-xl px-3 py-2 text-sm transition duration-150 focus-visible:shadow-glow focus-visible:outline-none',
                isActive
                  ? // 选中态：面板提亮 + 左侧金色指示条（而不是只变文字颜色）
                    'bg-teyvat-raised font-semibold text-teyvat-text before:absolute before:top-1/2 before:left-1 before:h-4 before:w-[3px] before:-translate-y-1/2 before:rounded-full before:bg-teyvat-gold before:shadow-glow'
                  : 'text-teyvat-text-2 hover:bg-white/5 hover:text-teyvat-text',
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
