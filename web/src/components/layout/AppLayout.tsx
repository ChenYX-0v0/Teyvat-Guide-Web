import { useEffect, useState } from 'react'
import { Outlet, useLocation } from 'react-router-dom'
import Header from './Header'
import MobileNav from './MobileNav'
import Sidebar from './Sidebar'
import SiteFooter from './SiteFooter'
import TermsModal from '@/components/legal/TermsModal'
import { useAuthStore } from '@/store/useAuthStore'
import { trackRouteChange } from '@/lib/track'

export default function AppLayout() {
  const verify = useAuthStore((s) => s.verify)
  const termsAcceptedVersion = useAuthStore((s) => s.termsAcceptedVersion)
  const termsCurrentVersion = useAuthStore((s) => s.termsCurrentVersion)
  const termsLoaded = useAuthStore((s) => s.termsLoaded)
  const [navOpen, setNavOpen] = useState(false)
  const location = useLocation()

  // 埋点：路由变化 → 上一次的 page_leave（含停留时长）+ 本次 page_view
  useEffect(() => {
    trackRouteChange(location.pathname)
  }, [location.pathname])

  // 启动时校验本地 Token：过期/失效则清凭据回登录页，避免"以为登录了其实在匿名聊天"
  useEffect(() => {
    void verify()
  }, [verify])

  // 用户须知强制确认：版本与已同意版本不一致时**不渲染任何页面内容**，
  // 只渲染阻断式弹窗（合规要求"每个用户都必须阅读"；版本真源在主服务 service/terms.go）。
  // termsLoaded 为 false 时不弹：避免启动瞬间拿旧状态误弹一下。
  const needTerms =
    termsLoaded && termsCurrentVersion !== '' && termsAcceptedVersion !== termsCurrentVersion
  if (needTerms) return <TermsModal version={termsCurrentVersion} />

  return (
    // h-dvh：移动端浏览器地址栏收起/展开时按动态视口取高，避免 h-full 导致的高度跳动与底部裁切
    <div className="flex h-dvh flex-col">
      <Header onMenuClick={() => setNavOpen(true)} />
      <div className="flex min-h-0 flex-1">
        <Sidebar />
        <main className="min-w-0 flex-1 overflow-y-auto">
          <Outlet />
        </main>
      </div>
      {/* 备案号（合规必需，见组件内注释与 web/11 §6.4）；放在滚动区之外 → 任何页面都常显 */}
      <SiteFooter />
      {/* 抽屉放在 Header 之外：Header 的 backdrop-blur 会成为 fixed 后代的包含块 */}
      <MobileNav open={navOpen} onClose={() => setNavOpen(false)} />
    </div>
  )
}
