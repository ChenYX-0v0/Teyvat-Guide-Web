import { createBrowserRouter } from 'react-router-dom'
import AppLayout from '@/components/layout/AppLayout'
import HomePage from '@/pages/HomePage'
import ChatPage from '@/pages/ChatPage'
import AccountPage from '@/pages/AccountPage'
import WikiPage from '@/pages/WikiPage'
import NotFoundPage from '@/pages/NotFoundPage'

export const router = createBrowserRouter([
  {
    path: '/',
    element: <AppLayout />,
    children: [
      { index: true, element: <HomePage /> },
      { path: 'chat', element: <ChatPage /> },
      { path: 'account', element: <AccountPage /> },
      { path: 'wiki', element: <WikiPage /> },
      { path: '*', element: <NotFoundPage /> },
    ],
  },
])
