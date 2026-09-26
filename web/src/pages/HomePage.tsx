import { Link } from 'react-router-dom'
import { useEffect, useState } from 'react'
import { ArrowRight, Map, MessageSquare, UserRound } from 'lucide-react'
import { chatApi } from '@/api/chat'
import { t } from '@/i18n'
import Card from '@/components/common/Card'
import { buttonClass } from '@/utils/buttonStyles'
import { EVENTS, track } from '@/lib/track'
import type { HealthData } from '@/types'

/** 功能卡：图标徽记用元素色点一下（小面积，不做大面积铺色） */
const features = [
  {
    key: 'chat',
    icon: MessageSquare,
    accent: 'var(--color-el-hydro)',
    title: t('home.feature1Title'),
    desc: t('home.feature1Desc'),
  },
  {
    key: 'profile',
    icon: UserRound,
    accent: 'var(--color-el-geo)',
    title: t('home.feature2Title'),
    desc: t('home.feature2Desc'),
  },
  {
    key: 'route',
    icon: Map,
    accent: 'var(--color-el-anemo)',
    title: t('home.feature3Title'),
    desc: t('home.feature3Desc'),
  },
]

export default function HomePage() {
  const [health, setHealth] = useState<HealthData>()

  useEffect(() => {
    chatApi.health().then(setHealth).catch(() => setHealth(undefined))
  }, [])

  return (
    <div className="mx-auto max-w-4xl px-4 py-10 sm:px-6 sm:py-14">
      {/* 英雄区：定位一句话 + 主 CTA */}
      <p className="text-xs tracking-[0.24em] text-teyvat-text-4">TEYVAT GUIDE</p>
      <h2 className="mt-3 text-2xl font-bold tracking-wide text-teyvat-gold sm:text-3xl">
        {t('home.welcome')}
      </h2>
      <p className="mt-3 max-w-2xl text-sm leading-relaxed text-teyvat-text-2">{t('home.intro')}</p>

      <div className="mt-8 flex flex-wrap items-center gap-3">
        <Link
          to="/chat"
          onClick={() => track(EVENTS.homeCtaClick, { target: 'chat' })}
          className={buttonClass({ variant: 'primary' })}
        >
          {t('home.startChat')}
          <ArrowRight className="h-4 w-4" />
        </Link>
        <Link
          to="/wiki"
          onClick={() => track(EVENTS.homeCtaClick, { target: 'wiki' })}
          className={buttonClass({ variant: 'ghost' })}
        >
          {t('home.viewWiki')}
        </Link>
      </div>

      <div className="mt-4 text-xs text-teyvat-text-4">
        {t('home.backendStatus')}
        {health ? (
          <span className="text-ok">
            {' '}
            {t('home.statusUp')}（{health.version}）
          </span>
        ) : (
          <span className="text-warn"> {t('home.statusDown')}</span>
        )}
      </div>

      <div className="mt-10 grid gap-4 sm:grid-cols-3">
        {features.map(({ key, icon: Icon, accent, title, desc }) => (
          <Card key={key} lift className="p-5">
            <span
              className="mb-3 inline-flex h-9 w-9 items-center justify-center rounded-xl border border-teyvat-line bg-white/[0.03]"
              style={{ color: accent }}
            >
              <Icon className="h-4 w-4" />
            </span>
            <h3 className="font-medium text-teyvat-text">{title}</h3>
            <p className="mt-2 text-xs leading-relaxed text-teyvat-text-3">{desc}</p>
          </Card>
        ))}
      </div>
    </div>
  )
}
