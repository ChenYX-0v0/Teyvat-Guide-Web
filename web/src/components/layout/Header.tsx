import { Compass } from 'lucide-react'
import { t } from '@/i18n'

export default function Header() {
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
      <span className="rounded-full border border-teyvat-gold/30 bg-teyvat-gold/10 px-3 py-1 text-xs text-teyvat-gold">
        v0.1.0
      </span>
    </header>
  )
}
