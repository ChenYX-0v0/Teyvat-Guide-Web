import HomeView from '@/components/home/HomeView'
import { t } from '@/i18n'
import { EVENTS, track } from '@/lib/track'

/**
 * 首页（登录区）。
 *
 * 2026-09-28：原「智能问答 / 档案查询 / 路线规划」三张功能卡**已移除** ✗ —— 它们只是把左侧导航又抄了一遍；
 * 同日加过的「接下来会有什么」板块也撤掉了 ✓（计划只留在 `teyvat-docs/web/06-开发路线图.md`）。
 * 2026-10-16：按用户要求**恢复进度板块并升级为三态**（已上线 / 开发中 / 待排期）✓，
 * 定位从"功能入口"改成"**开发进度公示**" ✓。
 * 2026-10-17：正文抽到 `components/home/HomeView` ✓ —— 演示区（`/demo`）要展示**同一张首页** ✓，
 * 这里只剩"登录区的主 CTA 与埋点" ✓。
 */
export default function HomePage() {
  return (
    <HomeView
      ctaTo="/chat"
      ctaLabel={t('home.startChat')}
      onCtaClick={() => track(EVENTS.homeCtaClick, { target: 'chat' })}
    />
  )
}
