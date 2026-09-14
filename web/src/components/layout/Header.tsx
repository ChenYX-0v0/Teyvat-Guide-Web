import { Compass, LogOut, Menu } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '@/store/useAuthStore'
import { t } from '@/i18n'

type Props = {
  /** 移动端汉堡按钮：由 AppLayout 控制抽屉开关 */
  onMenuClick: () => void
}

export default function Header({ onMenuClick }: Props) {
  const displayName = useAuthStore((s) => s.displayName)
  const logout = useAuthStore((s) => s.logout)
  const navigate = useNavigate()

  const handleLogout = () => {
    logout()
    navigate('/login', { replace: true })
  }

  return (
    <header className="flex items-center justify-between gap-3 border-b border-white/10 bg-black/20 px-4 py-3 backdrop-blur sm:px-6">
      <div className="flex min-w-0 items-center gap-2 sm:gap-3">
        <button
          type="button"
          onClick={onMenuClick}
          aria-label={t('header.menu')}
          className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border border-white/10 text-white/60 transition hover:text-white md:hidden"
        >
          <Menu className="h-4 w-4" />
        </button>
        <Compass className="h-6 w-6 shrink-0 text-teyvat-gold" />
        <div className="min-w-0">
          <h1 className="truncate text-base font-semibold tracking-wide text-teyvat-gold sm:text-lg">
            {t('header.title')}
          </h1>
          <p className="hidden truncate text-xs text-white/45 sm:block">{t('header.subtitle')}</p>
        </div>
      </div>

      <div className="flex shrink-0 items-center gap-2 sm:gap-3">
        <span className="max-w-[6rem] truncate rounded-full border border-teyvat-gold/30 bg-teyvat-gold/10 px-3 py-1 text-xs text-teyvat-gold sm:max-w-none">
          {displayName || 'Traveler'}
        </span>
        <button
          onClick={handleLogout}
          title={t('header.logout')}
          className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-white/10 text-white/50 transition hover:border-red-400/40 hover:text-red-300"
        >
          <LogOut className="h-4 w-4" />
        </button>
      </div>
    </header>
  )
}
