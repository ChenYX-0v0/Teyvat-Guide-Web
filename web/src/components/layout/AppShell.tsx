import { useEffect, useState, type ReactNode } from 'react'
import { useLocation } from 'react-router-dom'
import Header from './Header'
import MobileNav from './MobileNav'
import Sidebar from './Sidebar'
import SiteFooter from './SiteFooter'
import type { AsideRender, NavItem } from './navItems'
import { trackRouteChange } from '@/lib/track'
import { t } from '@/i18n'

interface Props {
  /** 导航项：登录区 `navItems` / 演示区 `demoNavItems` ✓ */
  navItems: NavItem[]
  /** 侧栏（桌面）与抽屉（移动）底部那块 ✓ */
  aside?: AsideRender
  /** 顶栏标题右侧附加块（演示页放「演示」徽标 ✓） */
  headerExtra?: ReactNode
  children: ReactNode
}

/**
 * 应用**纯外壳**（2026-10-17 抽出，见 TODO-32 ✓）—— 只负责"长得一样"，不含任何鉴权。
 *
 * 为什么要把外壳从 `AppLayout` 里拆出来 ✗→✓：
 * 演示页（`/demo`）是**免登录**的，但访客先看到的往往是它 ✓；此前它自己写了一套
 * "顶栏 + 顶部页签"的壳子，登录后再看真实页（侧栏 + 抽屉 + 底部用户卡）就觉得换了产品 ✗。
 * 拆开以后：骨架、宽度、导航形态、选中态、页脚只有一处实现 ✓，两边共用 ✓，
 * 日后改版也不会再漂移 ✓。
 *
 * 鉴权层（`RequireAuth` / `verify()` / 强制须知弹窗）留在 `AppLayout` ✓ ——
 * 那三件事放进通用外壳会让演示页把访客踢回登录页 ✗。
 */
export default function AppShell({ navItems, aside, headerExtra, children }: Props) {
  const [navOpen, setNavOpen] = useState(false)
  const location = useLocation()

  // 埋点：路由变化 → 上一次的 page_leave（含停留时长）+ 本次 page_view
  // （放在外壳里 ⇒ 登录区与演示区同一套口径 ✓，不用两边各写一遍 ✗）
  useEffect(() => {
    trackRouteChange(location.pathname)
  }, [location.pathname])

  return (
    // h-dvh：移动端浏览器地址栏收起/展开时按动态视口取高，避免 h-full 导致的高度跳动与底部裁切
    <div className="flex h-dvh flex-col">
      {/* 跳转链接（2026-10-18）：键盘 / 屏读用户不必逐个 Tab 过侧栏才能到正文。
          平时 sr-only，聚焦时才出现 —— 这是可达性硬要求，不是装饰 */}
      <a
        href="#main"
        className="sr-only focus:not-sr-only focus:fixed focus:top-3 focus:left-3 focus:z-[60] focus:rounded-lg focus:border focus:border-teyvat-line-strong focus:bg-teyvat-panel focus:px-3 focus:py-2 focus:text-xs focus:text-teyvat-text focus:shadow-glow"
      >
        {t('common.skipToContent')}
      </a>
      <Header onMenuClick={() => setNavOpen(true)} extra={headerExtra} />
      <div className="flex min-h-0 flex-1">
        <Sidebar items={navItems} aside={aside} />
        <main id="main" tabIndex={-1} className="min-w-0 flex-1 overflow-y-auto focus-visible:outline-none">
          {children}
        </main>
      </div>
      {/* 备案号（合规必需，见组件内注释与 web/11 §6.4）；放在滚动区之外 → 任何页面都常显 */}
      <SiteFooter />
      {/* 抽屉放在 Header 之外：Header 的 backdrop-blur 会成为 fixed 后代的包含块 */}
      <MobileNav open={navOpen} onClose={() => setNavOpen(false)} items={navItems} aside={aside} />
    </div>
  )
}
