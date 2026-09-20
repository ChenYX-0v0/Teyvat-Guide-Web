import { useNavigate } from 'react-router-dom'
import Button from '@/components/common/Button'
import { t } from '@/i18n'

export default function NotFoundPage() {
  const navigate = useNavigate()

  return (
    <div className="flex h-full flex-col items-center justify-center gap-4">
      <p className="text-5xl font-semibold tracking-wide text-teyvat-gold">{t('notFound.title')}</p>
      <p className="text-sm text-teyvat-text-3">{t('notFound.desc')}</p>
      <Button variant="ghost" onClick={() => navigate('/')}>
        {t('notFound.back')}
      </Button>
    </div>
  )
}
