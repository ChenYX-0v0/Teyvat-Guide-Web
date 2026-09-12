import { useEffect } from 'react'
import { Outlet } from 'react-router-dom'
import Header from './Header'
import Sidebar from './Sidebar'
import { useAuthStore } from '@/store/useAuthStore'

export default function AppLayout() {
  const verify = useAuthStore((s) => s.verify)

  // 启动时校验本地 Token：过期/失效则清凭据回登录页，避免"以为登录了其实在匿名聊天"
  useEffect(() => {
    void verify()
  }, [verify])

  return (
    <div className="flex h-full flex-col">
      <Header />
      <div className="flex min-h-0 flex-1">
        <Sidebar />
        <main className="min-w-0 flex-1 overflow-y-auto">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
