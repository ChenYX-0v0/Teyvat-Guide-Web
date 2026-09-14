import { useEffect } from 'react'
import { NavLink, useLocation } from 'react-router-dom'
import { X } from 'lucide-react'
import clsx from 'clsx'
import { navItems } from './navItems'
import { t } from '@/i18n'

type Props = {
  open: boolean
  onClose: () => void
}

/**
 * 移动端导航抽屉（<md 使用；桌面用 Sidebar）。
 * 注意：**必须渲染在 Header 之外**——Header 有 `backdrop-blur`，
 * backdrop-filter 会成为 fixed 后代的包含块，抽屉会被关在 header 内部。
 */
export default function MobileNav({ open, onClose }: Props) {
  const location = useLocation()

  // 路由变化后自动关闭（点导航项、或代码里跳转）
  useEffect(() => {
    onClose()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [location.pathname])

  // Esc 关闭 + 打开期间锁背景滚动
  useEffect(() => {
    if (!open) return
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKeyDown)
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      window.removeEventListener('keydown', onKeyDown)
      document.body.style.overflow = previousOverflow
    }
  }, [open, onClose])

  if (!open) return null

  return (
    <div className="fixed inset-0 z-50 touch-none md:hidden" role="dialog" aria-modal="true">
      <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" onClick={onClose} />
      <nav className="absolute inset-y-0 left-0 flex w-64 max-w-[80vw] flex-col border-r border-white/10 bg-teyvat-ink px-3 py-4 shadow-2xl">
        <div className="mb-3 flex items-center justify-between px-2">
          <span className="text-xs font-medium text-white/50">{t('header.menu')}</span>
          <button
            type="button"
            onClick={onClose}
            aria-label={t('header.close')}
            className="flex h-8 w-8 items-center justify-center rounded-lg text-white/50 transition hover:bg-white/5 hover:text-white"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        <div className="flex flex-col gap-1">
          {navItems.map(({ to, label, icon: Icon, end }) => (
            <NavLink
              key={to}
              to={to}
              end={end}
              onClick={onClose}
              className={({ isActive }) =>
                clsx(
                  'flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm transition',
                  isActive
                    ? 'bg-teyvat-gold/15 text-teyvat-gold'
                    : 'text-white/70 hover:bg-white/5 hover:text-white',
                )
              }
            >
              <Icon className="h-4 w-4" />
              {label}
            </NavLink>
          ))}
        </div>
      </nav>
    </div>
  )
}
