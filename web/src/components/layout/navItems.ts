import { House, MessagesSquare, UserRound } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { t } from '@/i18n'

export type NavItem = {
  to: string
  label: string
  icon: LucideIcon
  /** 精确匹配（首页需要，见 NavLink end） */
  end: boolean
}

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
