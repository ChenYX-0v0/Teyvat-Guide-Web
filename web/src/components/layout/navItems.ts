import { BookOpen, House, MessagesSquare, UserRound } from 'lucide-react'
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
  { to: '/wiki', label: t('sidebar.wiki'), icon: BookOpen, end: false },
]
