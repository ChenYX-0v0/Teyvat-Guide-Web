import { Compass, LogOut } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '@/store/useAuthStore'
import { t } from '@/i18n'

export default function Header() {
  const displayName = useAuthStore((s) => s.displayName)
  const logout = useAuthStore((s) => s.logout)
  const navigate = useNavigate()

  const handleLogout = () => {
    logout()
    navigate('/login', { replace: true })
  }

  return (
    <header className="flex items-center justify-between border-b border-white/10 bg-black/20 px-6 py-3 backdrop-blur">
      <div className="flex items-center gap-3">
        <Compass className="h-6 w-6 text-teyvat-gold" />
        <div>
          <h1 className="text-lg font-semibold tracking-wide text-teyvat-gold">
            {t('header.title')}
          </h1>
          <p className="text-xs text-white/45">{t('header.subtitle')}</p>
        </div>
      </div>
      <div className="flex items-center gap-3">
        <span className="rounded-full border border-teyvat-gold/30 bg-teyvat-gold/10 px-3 py-1 text-xs text-teyvat-gold">
          {displayName || 'Traveler'}
        </span>
        <button
          onClick={handleLogout}
          title={t('header.logout')}
          className="flex h-8 w-8 items-center justify-center rounded-lg border border-white/10 text-white/50 transition hover:border-red-400/40 hover:text-red-300"
        >
          <LogOut className="h-4 w-4" />
        </button>
      </div>
    </header>
  )
}
