import { Link } from 'react-router-dom'
import { useEffect, useState } from 'react'
import { ArrowRight } from 'lucide-react'
import { chatApi } from '@/api/chat'
import { t } from '@/i18n'
import { buttonClass } from '@/utils/buttonStyles'
import { EVENTS, track } from '@/lib/track'
import type { HealthData } from '@/types'

/**
 * 首页
 *
 * 2026-09-28：原「智能问答 / 档案查询 / 路线规划」三张功能卡**已移除** ✗ ——
 * 它们只是把左侧导航又抄了一遍（三处入口导航里本来就有），占着首屏却没给出新信息。
 *
 * 腾出的位置留给规划中的三张新卡（口径与依赖见 `teyvat-docs/web/06-开发路线图.md` P2）：
 *   1) 推荐优先培养角色  2) 当前祈愿 + 抽取建议  3) 主线剧情回顾
 * 在它们落地前，首页 = 英雄区（定位 + 主/次 CTA）+ 后端状态行。
 */
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
    </div>
  )
}
