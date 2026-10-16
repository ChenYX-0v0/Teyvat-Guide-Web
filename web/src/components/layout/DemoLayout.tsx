import { useEffect } from 'react'
import { Link, NavLink, Outlet, useLocation } from 'react-router-dom'
import clsx from 'clsx'
import { Compass } from 'lucide-react'
import Chip from '@/components/common/Chip'
import SiteFooter from './SiteFooter'
import { buttonClass } from '@/utils/buttonStyles'
import { trackRouteChange } from '@/lib/track'
import { t } from '@/i18n'

/**
 * 演示模式外壳（2026-10-17，**公开**，不含任何鉴权）。
 *
 * 为什么不复用 `AppLayout` ✗：那是登录后的外壳——外面套着 `RequireAuth`、内部会 `verify()`
 * 校验 Token、还会弹强制阅读的用户须知。演示页是"给还没注册的人看的"✓，
 * 这三件事全都不该发生 ✓（否则游客一进来就被踢回登录页，演示也就没人看得到 ✗）。
 *
 * 所以这里只保留演示页真正需要的三块：品牌头（+ 演示徽标 + 登录入口）、两个页签、页脚备案号 ✓。
 */
const tabs = [
  { to: '/demo', label: t('demo.tabAccount'), end: true },
  { to: '/demo/chat', label: t('demo.tabChat'), end: false },
]

export default function DemoLayout() {
  const location = useLocation()

  // 埋点：演示页也在 AppLayout 之外，路由浏览得自己上报 ✓（事件名与登录区同一套，不新增 ✓）
  useEffect(() => {
    trackRouteChange(location.pathname)
  }, [location.pathname])

  return (
    <div className="flex h-dvh flex-col">
      <header className="flex items-center justify-between gap-3 border-b border-teyvat-line bg-black/20 px-4 py-3 backdrop-blur sm:px-6">
        <div className="flex min-w-0 items-center gap-2 sm:gap-3">
          <Compass className="h-6 w-6 shrink-0 text-teyvat-gold" />
          <div className="min-w-0">
            <h1 className="truncate text-base font-semibold tracking-wide text-teyvat-gold sm:text-lg">
              {t('header.title')}
            </h1>
            <p className="hidden truncate text-xs text-teyvat-text-3 sm:block">
              {t('header.subtitle')}
            </p>
          </div>
          {/* 徽标常显：让访客一眼看出"看到的是演示数据"，而不是某个真人的账号 ✓ */}
          <Chip tone="gold">{t('demo.badge')}</Chip>
        </div>

        <Link to="/login" className={buttonClass({ variant: 'primary', size: 'sm' })}>
          {t('demo.loginCta')}
        </Link>
      </header>

      <div className="flex min-h-0 flex-1 flex-col">
        <nav className="flex shrink-0 gap-1 border-b border-teyvat-line px-4 py-2 sm:px-6">
          {tabs.map(({ to, label, end }) => (
            <NavLink
              key={to}
              to={to}
              end={end}
              className={({ isActive }) =>
                clsx(
                  'rounded-lg px-3 py-1.5 text-sm transition duration-150 focus-visible:shadow-glow focus-visible:outline-none',
                  isActive
                    ? 'bg-teyvat-raised font-semibold text-teyvat-text'
                    : 'text-teyvat-text-2 hover:bg-white/5 hover:text-teyvat-text',
                )
              }
            >
              {label}
            </NavLink>
          ))}
        </nav>

        <main className="min-w-0 flex-1 overflow-y-auto">
          <Outlet />
        </main>
      </div>

      {/* 演示页同样在 AppLayout 之外 ⇒ 备案号必须自带（公安审核可能第一屏就落在公开页，见 web/11 §6.5） */}
      <SiteFooter />
    </div>
  )
}
