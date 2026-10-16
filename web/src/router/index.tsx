import { createBrowserRouter, Navigate } from 'react-router-dom'
import AppLayout from '@/components/layout/AppLayout'
import DemoLayout from '@/components/layout/DemoLayout'
import LoginPage from '@/pages/LoginPage'
import HomePage from '@/pages/HomePage'
import ChatPage from '@/pages/ChatPage'
import AccountPage from '@/pages/AccountPage'
import DemoAccountPage from '@/pages/DemoAccountPage'
import DemoChatPage from '@/pages/DemoChatPage'
import WikiPage from '@/pages/WikiPage'
import TermsPage from '@/pages/TermsPage'
import NotFoundPage from '@/pages/NotFoundPage'
import { useAuthStore } from '@/store/useAuthStore'

/** 未登录访问受保护页面时跳转登录页 */
function RequireAuth({ children }: { children: React.ReactNode }) {
  const token = useAuthStore((s) => s.token)
  if (!token) {
    return <Navigate to="/login" replace />
  }
  return children
}

export const router = createBrowserRouter([
  { path: '/login', element: <LoginPage /> },
  // 用户须知：**公开**（不登录也能读）——审核人员/新用户先看到条款，且页脚有常驻入口
  { path: '/terms', element: <TermsPage /> },
  // 游客演示模式（2026-10-17）：**公开**（不登录也能看）——给"还没注册的人"先看清楚产品长什么样 ✓。
  // 内容全部取自库里预置数据（`GET /api/demo/showcase` + 预设问答），**不调模型、不消耗 token** ✓；
  // 外壳用 DemoLayout（不是 AppLayout）⇒ 不触发 Token 校验，也不弹强制阅读的用户须知 ✓。
  {
    path: '/demo',
    element: <DemoLayout />,
    children: [
      { index: true, element: <DemoAccountPage /> },
      { path: 'chat', element: <DemoChatPage /> },
    ],
  },
  {
    path: '/',
    element: (
      <RequireAuth>
        <AppLayout />
      </RequireAuth>
    ),
    children: [
      { index: true, element: <HomePage /> },
      { path: 'chat', element: <ChatPage /> },
      { path: 'account', element: <AccountPage /> },
      { path: 'wiki', element: <WikiPage /> },
      { path: '*', element: <NotFoundPage /> },
    ],
  },
])
