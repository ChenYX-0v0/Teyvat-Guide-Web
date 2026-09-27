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
 * 2026-09-28：
 * - 原「智能问答 / 档案查询 / 路线规划」三张功能卡**已移除** ✗ —— 它们只是把左侧导航又抄了一遍，
 *   占着首屏却没给出新信息；取而代之曾加过一块「接下来会有什么」开发计划板块，
 *   **同日也撤掉了** ✓（用户看过觉得没必要 ⇒ 计划只留在 `teyvat-docs/web/06-开发路线图.md` 里 ✓）。
 * - 图鉴也从菜单撤下（占位页；**路由保留** ⇒ 直接输 /wiki 仍可访问 ✓）。
 *
 * 所以首页当前就是：英雄区（定位 + 主 CTA）+ 后端状态行。
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
