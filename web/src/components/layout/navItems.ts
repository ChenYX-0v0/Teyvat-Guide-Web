import { House, MessagesSquare, UserRound } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import type { ReactNode } from 'react'
import { t } from '@/i18n'

export type NavItem = {
  to: string
  label: string
  icon: LucideIcon
  /** 精确匹配（首页需要，见 NavLink end） */
  end: boolean
}

/**
 * 侧栏 / 抽屉**底部区块**的渲染上下文。
 *
 * 为什么是函数而不是节点 ✗：真实页底部是 `UserCard`，它在抽屉里需要"点完顺手关掉抽屉" ✓
 * （`onNavigate`），而桌面侧栏不需要 —— 一个现成节点没法同时满足两边 ✓。
 */
export type AsideRender = (ctx: { onNavigate?: () => void }) => ReactNode

/**
 * 主导航配置：桌面侧栏（Sidebar）与移动端抽屉（MobileNav）共用这一份。
 * 抽出来的原因：移动端曾经完全没有导航入口，如果两边各写一份 items，
 * 以后加页面必然出现"桌面有、手机没有"的漂移。
 */
export const navItems: NavItem[] = [
  { to: '/', label: t('sidebar.home'), icon: House, end: true },
  { to: '/chat', label: t('sidebar.chat'), icon: MessagesSquare, end: false },
  { to: '/account', label: t('sidebar.account'), icon: UserRound, end: false },
  // 2026-09-28：图鉴（原第 4 项 + BookOpen 图标）**先从菜单撤下** ✗ ——
  // 它至今是占位页（`WikiPage` 里是假数据 + 「后续接入真实数据」文案），挂在导航上只是占位，
  // 用户判断"目前也没有用" ✓。路由**保留**（直接输 /wiki 仍可访问 ✓），做真实数据时把这一行、
  // 上面的 BookOpen 导入、i18n 的 sidebar.wiki（zh/en）一起恢复 ✓。
  // 一起撤的还有首页那个「查看图鉴」次 CTA（`HomePage.tsx`）✓ —— 否则首页会指着一个刚被藏起来的页面 ✗。
  // { to: '/wiki', label: t('sidebar.wiki'), icon: BookOpen, end: false },
]

/**
 * 演示模式的导航项（2026-10-17 ✓，见 TODO-32）。
 *
 * 与登录区**共用同一份外壳**（`AppShell` ✓）：侧栏位置、宽度、图标、选中态全都一致 ✓，
 * 只换导航项与底部区块 —— 访客先看演示页、登录后再看真实页，不该有"换了产品"的错觉 ✗。
 */
export const demoNavItems: NavItem[] = [
  // 顺序与登录区一致：首页 → 对话 → 账号 ✓（演示区补了首页之后，访客进来先看到"这是什么产品" ✓）
  // 演示首页是 /demo 的 index 路由 ⇒ 必须 end 精确匹配，否则在 /demo/chat 上两项会同时高亮 ✗
  { to: '/demo', label: t('demo.tabHome'), icon: House, end: true },
  { to: '/demo/chat', label: t('demo.tabChat'), icon: MessagesSquare, end: false },
  { to: '/demo/account', label: t('demo.tabAccount'), icon: UserRound, end: false },
]
