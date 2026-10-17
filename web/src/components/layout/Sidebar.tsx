import { NavLink } from 'react-router-dom'
import clsx from 'clsx'
import { navItems, type AsideRender, type NavItem } from './navItems'
import UserCard from './UserCard'

type Props = {
  /** 导航项：登录区是 `navItems`，演示区是 `demoNavItems` ✓ */
  items?: NavItem[]
  /** 底部区块：不传就是登录区的 `UserCard` ✓ */
  aside?: AsideRender
}

/**
 * 桌面端常驻侧栏（<md 不渲染，移动端用 MobileNav 抽屉）。
 *
 * 2026-10-11：改成**列布局**，底部固定一块用户卡片（`UserCard` ✓）——
 * `mt-auto` 把它压到底部 ✓，卡片点开是"个人信息管理 / 修改密码 / 退出登录" ✓。
 *
 * 2026-10-17：导航项与底部区块改成**外部传入** ✓ —— 演示模式（`/demo`）要复用同一个侧栏，
 * 只换两项导航和底部那块说明卡 ✓（此前演示页走的是自己写的一套顶部页签，见 TODO-32 ✗）。
 */
export default function Sidebar({ items = navItems, aside }: Props) {
  return (
    <aside className="hidden w-56 shrink-0 border-r border-teyvat-line bg-black/20 p-3 md:flex md:flex-col">
      <nav className="flex flex-col gap-1">
        {items.map(({ to, label, icon: Icon, end }) => (
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

      {/* 底部一栏：贴侧栏底部（mt-auto 在 UserCard / 演示说明卡内部 ✓） */}
      {aside ? aside({}) : <UserCard />}
    </aside>
  )
}
