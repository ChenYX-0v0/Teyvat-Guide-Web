import { useEffect } from 'react'
import { Outlet } from 'react-router-dom'
import AppShell from './AppShell'
import UserCard from './UserCard'
import { navItems } from './navItems'
import TermsModal from '@/components/legal/TermsModal'
import { useAuthStore } from '@/store/useAuthStore'

/**
 * 登录区外壳 = **鉴权层 ＋ 纯外壳**（2026-10-17 拆分，见 TODO-32 ✓）。
 *
 * 这里只剩下"登录区专属"的三件事 ✓：
 *  1. 启动校验本地 Token（过期/失效清凭据回登录页 ✓）；
 *  2. 用户须知强制确认（版本不一致时只渲染阻断弹窗 ✓）；
 *  3. 装入登录区的导航项与底部用户卡片 ✓。
 * 骨架、导航形态、页脚、路由埋点都在 `AppShell` 里 ✓ —— 与演示区（`DemoLayout`）共用同一份 ✓。
 */
export default function AppLayout() {
  const verify = useAuthStore((s) => s.verify)
  const termsAcceptedVersion = useAuthStore((s) => s.termsAcceptedVersion)
  const termsCurrentVersion = useAuthStore((s) => s.termsCurrentVersion)
  const termsLoaded = useAuthStore((s) => s.termsLoaded)

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
    <AppShell navItems={navItems} aside={({ onNavigate }) => <UserCard onNavigate={onNavigate} />}>
      <Outlet />
    </AppShell>
  )
}
