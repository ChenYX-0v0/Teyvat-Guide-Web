import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { RouterProvider } from 'react-router-dom'
import { router } from './router'
import { initTracking } from './lib/track'
import './index.css'

// 埋点：接管首次 page_view、切后台的 page_leave 与前端异常（shared-docs/07）
initTracking()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <RouterProvider router={router} />
  </StrictMode>,
)
