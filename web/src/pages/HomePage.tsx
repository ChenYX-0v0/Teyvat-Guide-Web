import { Link } from 'react-router-dom'
import { useEffect, useState } from 'react'
import { ArrowRight } from 'lucide-react'
import { chatApi } from '@/api/chat'
import { t } from '@/i18n'
import { buttonClass } from '@/utils/buttonStyles'
import { EVENTS, track } from '@/lib/track'
import Card from '@/components/common/Card'
import Chip from '@/components/common/Chip'
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
 * 2026-10-16：按用户要求**恢复进度板块并升级为三态**（已上线 / 开发中 / 待排期）✓。
 * 定位从"功能入口"改成"**开发进度公示**"——给自己记学习进度，也给来帮忙的朋友一眼看清现状 ✓。
 *
 * ⚠️ 三条口径（沿用 2026-09-28 首版留档 + 本次明确）：
 * 1. 内容**硬编码在下面的 `PLAN` 数组**里（文案走 i18n，**不入库**）✓ —— 记的是产品进度文案，
 *    不是资产映射 / 枚举那类业务数据，与 `AGENTS.md` §4.4 / §7.13 不冲突 ✓。
 * 2. 措辞一律**使用者视角** ✓ —— 只说"你会得到什么"，不写 RAG / PWA / lint 之类内部词 ✗；
 *    **后台与工程类任务不进这里** ✗（它们留在 `web/06` 路线图）。
 * 3. key 必须写**完整字面量** ✓ —— `t()` 的入参是穷举字面量联合类型，用模板字符串拼
 *    （如 `home.plan.${key}`）会 **TS2345 直接构建失败** ✗（2026-09-28 已踩过 ✓）。
 *
 * ⚠️ 本次**未加埋点** ✓：新增事件要同步主服务 `usageEventProps` 白名单与后台 `EVENT_LABELS`
 * （跨层契约，见 `web/06` P3 门禁第 ④ 条），而这块目前没有分析需求 ✗。
 */

/**
 * 进度卡片：顺序 = 已上线 → 开发中 → 待排期。
 * 状态点沿用"红绿灯"语义，颜色只取 `index.css` 的 `@theme` token（禁止硬编码色值）。
 */
const PLAN = [
  {
    label: 'home.plan.online',
    dot: 'var(--color-ok)',
    items: [
      'home.plan.onlineItem1',
      'home.plan.onlineItem2',
      'home.plan.onlineItem3',
      'home.plan.onlineItem4',
      'home.plan.onlineItem5',
    ],
  },
  {
    label: 'home.plan.doing',
    dot: 'var(--color-warn)',
    items: ['home.plan.doingItem1', 'home.plan.doingItem2', 'home.plan.doingItem3'],
  },
  {
    label: 'home.plan.planned',
    dot: 'var(--color-teyvat-text-4)',
    items: [
      'home.plan.plannedItem1',
      'home.plan.plannedItem2',
      'home.plan.plannedItem3',
      'home.plan.plannedItem4',
      'home.plan.plannedItem5',
      'home.plan.plannedItem6',
      'home.plan.plannedItem7',
    ],
  },
] as const

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

      {/* 开发进度：三张状态卡（已上线 / 开发中 / 待排期） */}
      <section className="mt-10 sm:mt-12">
        <h3 className="text-sm font-semibold tracking-wide text-teyvat-text-2">
          {t('home.plan.title')}
        </h3>
        <p className="mt-1 text-xs text-teyvat-text-4">{t('home.plan.subtitle')}</p>

        <div className="mt-4 grid gap-4 md:grid-cols-3">
          {PLAN.map((card) => (
            <Card key={card.label} className="p-4">
              <Chip dot={card.dot}>{t(card.label)}</Chip>
              <ul className="mt-3 space-y-2">
                {card.items.map((key) => (
                  <li key={key} className="flex gap-2 text-sm text-teyvat-text-2">
                    <span className="mt-1.5 h-1 w-1 shrink-0 rounded-full bg-teyvat-text-4" />
                    {t(key)}
                  </li>
                ))}
              </ul>
            </Card>
          ))}
        </div>
      </section>
    </div>
  )
}
