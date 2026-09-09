import { useNavigate } from 'react-router-dom'
import { t } from '@/i18n'

export default function NotFoundPage() {
  const navigate = useNavigate()

  return (
    <div className="flex h-full flex-col items-center justify-center gap-4">
      <p className="text-5xl font-semibold text-teyvat-gold">{t('notFound.title')}</p>
      <p className="text-sm text-white/50">{t('notFound.desc')}</p>
      <button
        onClick={() => navigate('/')}
        className="rounded-lg border border-teyvat-gold/40 px-4 py-2 text-sm text-teyvat-gold transition hover:bg-teyvat-gold/10"
      >
        {t('notFound.back')}
      </button>
    </div>
  )
}
