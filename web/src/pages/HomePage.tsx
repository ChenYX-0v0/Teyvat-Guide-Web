import clsx from 'clsx'
import { Link } from 'react-router-dom'
import { useEffect, useState } from 'react'
import { ArrowRight } from 'lucide-react'
import { chatApi } from '@/api/chat'
import Card from '@/components/common/Card'
import { t } from '@/i18n'
import { buttonClass } from '@/utils/buttonStyles'
import { EVENTS, track } from '@/lib/track'
import type { HealthData } from '@/types'

/**
 * 首页「接下来会有什么」的条目 —— **硬编码在前端** ✓（这是给人看的开发计划）。
 *
 * ⚠️ 这里刻意写**完整的字面量 key**（而不是 `home.roadmap.${key}.title` 拼出来）✗ ——
 * `t()` 的入参是**穷举的字面量联合类型**，模板字符串拼出来的 key 编译不过 ✓
 * （2026-09-28 踩到：TS2345，构建直接失败 ✗）。
 *
 * 维护方式：新增一项 / 某项上线，就改这个数组 + i18n 里对应的 `home.roadmap.*.title/desc`
 * （zh-CN 与 en **必须同步** ✓，两边键数要对得上 ✓）。
 * 口径：**只用使用者视角的语言** ✓ —— 说"你会得到什么"，不写 RAG / PWA / Canvas 这类内部词 ✗；
 * 后台与工程类任务（埋点看板、CI、迁移工具等）不进这里 ✗，它们登记在 `teyvat-docs/web/06-开发路线图.md`。
 */
const ROADMAP = [
  { titleKey: 'home.roadmap.buildPriority.title', descKey: 'home.roadmap.buildPriority.desc', status: 'doing' },
  { titleKey: 'home.roadmap.banner.title', descKey: 'home.roadmap.banner.desc', status: 'doing' },
  { titleKey: 'home.roadmap.story.title', descKey: 'home.roadmap.story.desc', status: 'planned' },
  { titleKey: 'home.roadmap.wiki.title', descKey: 'home.roadmap.wiki.desc', status: 'planned' },
  { titleKey: 'home.roadmap.teamBuilder.title', descKey: 'home.roadmap.teamBuilder.desc', status: 'planned' },
  { titleKey: 'home.roadmap.farmRoute.title', descKey: 'home.roadmap.farmRoute.desc', status: 'planned' },
  { titleKey: 'home.roadmap.endgame.title', descKey: 'home.roadmap.endgame.desc', status: 'planned' },
  { titleKey: 'home.roadmap.theme.title', descKey: 'home.roadmap.theme.desc', status: 'planned' },
  { titleKey: 'home.roadmap.pwa.title', descKey: 'home.roadmap.pwa.desc', status: 'planned' },
] as const

/**
 * 首页
 *
 * 2026-09-28：原「智能问答 / 档案查询 / 路线规划」三张功能卡**已移除** ✗ ——
 * 它们只是把左侧导航又抄了一遍（三处入口导航里本来就有），占着首屏却没给出新信息。
 * 取而代之的是下面这块「接下来会有什么」（见 ROADMAP ✓）：只讲用户会得到什么 ✓。
 * 图鉴也从菜单撤下了（占位页；路由保留 ⇒ 直接输 /wiki 仍可访问 ✓）。
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

      {/* 开发计划 —— 措辞**必须站在使用者视角** ✓（只说"你会得到什么"，不用内部术语 ✗）。
          内容硬编码在前端 ✓：改 ROADMAP 数组 + i18n 文案（zh/en 同步 ✓）即可。 */}
      <section className="mt-14">
        <h3 className="text-sm font-medium text-teyvat-text-2">{t('home.roadmapTitle')}</h3>
        <p className="mt-1 text-xs text-teyvat-text-4">{t('home.roadmapHint')}</p>
        <div className="mt-4 grid gap-3 sm:grid-cols-2">
          {ROADMAP.map(({ titleKey, descKey, status }) => (
            <Card key={titleKey} className="p-4">
              <div className="flex items-start justify-between gap-3">
                <h4 className="text-sm font-medium text-teyvat-text">{t(titleKey)}</h4>
                <span
                  className={clsx(
                    'shrink-0 rounded-full border px-2 py-0.5 text-[10px] whitespace-nowrap',
                    status === 'doing'
                      ? 'border-teyvat-gold/40 bg-teyvat-gold/10 text-teyvat-gold'
                      : 'border-teyvat-line text-teyvat-text-4'
                  )}
                >
                  {status === 'doing' ? t('home.roadmapDoing') : t('home.roadmapPlanned')}
                </span>
              </div>
              <p className="mt-2 text-xs leading-relaxed text-teyvat-text-3">{t(descKey)}</p>
            </Card>
          ))}
        </div>
      </section>
    </div>
  )
}
