import { createBrowserRouter, Navigate } from 'react-router-dom'
import AppLayout from '@/components/layout/AppLayout'
import LoginPage from '@/pages/LoginPage'
import HomePage from '@/pages/HomePage'
import ChatPage from '@/pages/ChatPage'
import AccountPage from '@/pages/AccountPage'
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
