import { Link } from 'react-router-dom'
import { useEffect, useState } from 'react'
import { chatApi } from '@/api/chat'
import { t } from '@/i18n'
import type { HealthData } from '@/types'

const features = [
  { title: t('home.feature1Title'), desc: t('home.feature1Desc') },
  { title: t('home.feature2Title'), desc: t('home.feature2Desc') },
  { title: t('home.feature3Title'), desc: t('home.feature3Desc') },
]

export default function HomePage() {
  const [health, setHealth] = useState<HealthData>()

  useEffect(() => {
    chatApi.health().then(setHealth).catch(() => setHealth(undefined))
  }, [])

  return (
    <div className="mx-auto max-w-4xl px-6 py-14">
      <h2 className="text-3xl font-semibold tracking-wide text-teyvat-gold">
        {t('home.welcome')}
      </h2>
      <p className="mt-3 text-sm leading-relaxed text-white/60">{t('home.intro')}</p>

      <div className="mt-10 grid gap-4 sm:grid-cols-3">
        {features.map((f) => (
          <div
            key={f.title}
            className="rounded-2xl border border-white/10 bg-teyvat-panel/60 p-5 transition hover:border-teyvat-gold/40"
          >
            <h3 className="font-medium text-white">{f.title}</h3>
            <p className="mt-2 text-xs leading-relaxed text-white/55">{f.desc}</p>
          </div>
        ))}
      </div>

      <div className="mt-10 flex items-center gap-4">
        <Link
          to="/chat"
          className="rounded-xl bg-teyvat-gold px-5 py-2.5 text-sm font-medium text-teyvat-ink transition hover:bg-teyvat-gold-deep"
        >
          {t('home.startChat')}
        </Link>
        <span className="text-xs text-white/40">
          {t('home.backendStatus')}
          {health ? (
            <span className="text-emerald-400">
              {' '}
              {t('home.statusUp')}（{health.version}）
            </span>
          ) : (
            <span className="text-amber-400"> {t('home.statusDown')}</span>
          )}
        </span>
      </div>
    </div>
  )
}
