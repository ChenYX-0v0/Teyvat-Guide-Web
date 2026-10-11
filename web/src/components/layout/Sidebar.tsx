import { NavLink } from 'react-router-dom'
import clsx from 'clsx'
import { navItems } from './navItems'
import UserCard from './UserCard'

/**
 * 桌面端常驻侧栏（<md 不渲染，移动端用 MobileNav 抽屉）。
 *
 * 2026-10-11：改成**列布局**，底部固定一块用户卡片（`UserCard` ✓）——
 * `mt-auto` 把它压到底部 ✓，卡片点开是"个人信息管理 / 修改密码 / 退出登录" ✓。
 */
export default function Sidebar() {
  return (
    <aside className="hidden w-56 shrink-0 border-r border-teyvat-line bg-black/20 p-3 md:flex md:flex-col">
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

      {/* 用户一栏：贴侧栏底部（mt-auto 在 UserCard 内部 ✓） */}
      <UserCard />
    </aside>
  )
}
