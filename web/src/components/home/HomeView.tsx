import { useEffect, useState, type ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { ArrowRight } from 'lucide-react'
import { chatApi } from '@/api/chat'
import Card from '@/components/common/Card'
import Chip from '@/components/common/Chip'
import { buttonClass } from '@/utils/buttonStyles'
import { t } from '@/i18n'
import type { HealthData } from '@/types'

/**
 * 首页正文（英雄区 ＋ 主 CTA ＋ 后端状态 ＋ 开发进度三卡）—— **登录区与演示区共用一份** ✓。
 *
 * 2026-10-17 抽出（见 TODO-32 ✓）：演示区补了首页之后，两边必须"同一张脸" ✓；
 * 若各写一份，进度文案、状态行、卡片排版迟早漂移 ✗。
 *
 * 三条口径（沿用 2026-09-28 首版留档 ＋ 2026-10-16 三态改版）：
 * 1. 进度内容**硬编码在 `PLAN` 里**（文案走 i18n，**不入库**）✓ —— 记的是产品进度文案，
 *    不是资产映射 / 枚举那类业务数据，与 `AGENTS.md` §4.4 不冲突 ✓；
 * 2. 措辞一律**使用者视角** ✓ —— 只说"你会得到什么"，不写 RAG / PWA / lint 之类内部词 ✗；
 * 3. key 必须写**完整字面量** ✓ —— `t()` 的入参是穷举字面量联合类型，
 *    用模板字符串拼（如 `home.plan.${key}`）会 **TS2345 直接构建失败** ✗（2026-09-28 踩过 ✓）。
 */

/** 进度卡片：顺序 = 已上线 → 开发中 → 待排期。状态点沿用"红绿灯"语义，颜色只取 `@theme` token（禁止硬编码色值） */
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

interface Props {
  /** 主 CTA 指向哪儿：登录区 `/chat` ✓、演示区 `/demo/chat` ✓ */
  ctaTo: string
  ctaLabel: string
  /** 主 CTA 的埋点回调（演示区不传：不新增事件与属性值 ✓） */
  onCtaClick?: () => void
  /** 主 CTA 右侧的次按钮（演示区放"登录 / 注册" ✓） */
  extraCta?: ReactNode
  /** 状态行下方的补充说明（演示区放"内容是预置示例" ✓） */
  hint?: ReactNode
}

export default function HomeView({ ctaTo, ctaLabel, onCtaClick, extraCta, hint }: Props) {
  const [health, setHealth] = useState<HealthData>()

  // 后端状态：健康检查是**公开**接口 ✓，演示页同样能显示真实状态（不是假绿灯 ✗）
  useEffect(() => {
    chatApi
      .health()
      .then(setHealth)
      .catch(() => setHealth(undefined))
  }, [])

  return (
    <div className="mx-auto max-w-4xl px-4 py-10 sm:px-6 sm:py-14">
      {/* 英雄区：定位一句话 + 主 CTA */}
      <p className="text-[11px] tracking-[0.24em] text-teyvat-text-4">TEYVAT GUIDE</p>
      <h2 className="mt-3 text-2xl leading-tight font-semibold tracking-[0.01em] text-teyvat-gold sm:text-[32px]">
        {t('home.welcome')}
      </h2>

      <div className="mt-8 flex flex-wrap items-center gap-3">
        <Link to={ctaTo} onClick={onCtaClick} className={buttonClass({ variant: 'primary' })}>
          {ctaLabel}
          <ArrowRight className="h-4 w-4" />
        </Link>
        {extraCta}
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

      {hint}

      {/* 开发进度：三张状态卡（已上线 / 开发中 / 待排期） */}
      <section className="mt-12 sm:mt-16">
        <h3 className="text-[13px] font-semibold tracking-[0.04em] text-teyvat-text-2">
          {t('home.plan.title')}
        </h3>
        <p className="mt-1.5 text-xs leading-relaxed text-teyvat-text-4">{t('home.plan.subtitle')}</p>

        <div className="mt-5 grid gap-3 md:grid-cols-3">
          {PLAN.map((card) => (
            <Card key={card.label} className="p-5">
              <Chip dot={card.dot}>{t(card.label)}</Chip>
              <ul className="mt-4 space-y-2.5">
                {card.items.map((key) => (
                  <li key={key} className="flex gap-2 text-[13px] leading-relaxed text-teyvat-text-2">
                    <span className="mt-[7px] h-1 w-1 shrink-0 rounded-full bg-teyvat-text-4" />
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
