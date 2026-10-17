import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { Info, UserRound } from 'lucide-react'
import PlayerProfileView from '@/components/account/PlayerProfileView'
import DemoAnalysis from '@/components/account/DemoAnalysis'
import Alert from '@/components/common/Alert'
import EmptyState from '@/components/common/EmptyState'
import Skeleton from '@/components/common/Skeleton'
import { demoApi } from '@/api/demo'
import { buttonClass } from '@/utils/buttonStyles'
import { t } from '@/i18n'
import type { DemoShowcase } from '@/types/demo'
import type { PlayerProfile } from '@/types/genshin'

/**
 * 演示档案（2026-10-17，公开路由 `/demo`；同日二次改版对齐真实页，见 TODO-32 ✓）。
 *
 * 数据来自 `GET /api/demo/showcase`：一个**模拟旅行者账号头** + 4 位演示角色（各自的角色卡 + 预置「分析」）✓。
 *
 * 复用口径 ✓：演示角色的 `card` 与真实档案**同构** ⇒ 整个档案视图（角色条 / 面板 /
 * 圣遗物卡片 / 评分与套装统计）**一行都不用重写** ✓，直接交给 `PlayerProfileView`；
 * "分析"也复用真实页的**悬浮按钮 ＋ 浮动面板**（`DemoAnalysis` ✓），而不是另做一张页内卡 ✗。
 *
 * 保留的不同（**有理由** ✓）：顶部多一条演示提示条（合规：说明不是实时账号数据 ✓）、
 * 没有"查询 / 绑定 UID"卡（演示页没有账号可绑 ✓）—— 其余留白与真实账号页一致 ✓。
 */
export default function DemoAccountPage() {
  const [data, setData] = useState<DemoShowcase | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [selectedId, setSelectedId] = useState<number | null>(null)

  useEffect(() => {
    let alive = true
    demoApi
      .showcase()
      .then((d) => {
        if (alive) setData(d)
      })
      .catch((e: unknown) => {
        if (alive) setError(e instanceof Error ? e.message : t('errors.unknown'))
      })
      .finally(() => {
        if (alive) setLoading(false)
      })
    return () => {
      alive = false
    }
  }, [])

  /**
   * 演示数据 → 与真实档案同构的 `PlayerProfile`。
   *
   * ⚠️ `uid` 刻意留空 ✓：演示账号是编的、**没有游戏 UID**，
   * 编一个假的 UID 反而像"这是某个真人的账号" ✗（`PlayerProfileView` 对空 uid 不渲染那一行 ✓）。
   */
  const profile = useMemo<PlayerProfile | null>(() => {
    if (!data) return null
    return {
      source: 'demo',
      uid: '',
      nickname: data.profile?.nickname || t('demo.guestName'),
      level: data.profile?.level ?? 0,
      signature: data.profile?.signature ?? '',
      worldLevel: data.profile?.worldLevel ?? 0,
      achievements: data.profile?.achievements ?? 0,
      characters: data.characters.map((c) => c.card),
    }
  }, [data])

  // 选中角色由本页持有（与 AccountPage 同一套主从交互）；未选时落回第一位
  const selected = useMemo(() => {
    if (!data || data.characters.length === 0) return null
    return data.characters.find((c) => c.card.id === selectedId) ?? data.characters[0]
  }, [data, selectedId])

  if (loading) {
    return <DemoProfileSkeleton />
  }
  if (error) {
    return (
      <div className="mx-auto max-w-5xl px-4 pt-6 sm:px-6 sm:pt-8">
        <Alert tone="error">{error}</Alert>
      </div>
    )
  }
  if (!profile || !data) {
    return (
      <EmptyState
        icon={UserRound}
        title={t('demo.empty')}
        className="mx-auto max-w-5xl px-6 py-16"
      />
    )
  }

  return (
    // 留白与 AccountPage 一致（pb-24：给右下角悬浮按钮让位 ✓）
    <div className="mx-auto max-w-5xl space-y-6 px-4 pt-6 pb-24 sm:px-6 sm:pt-8 sm:pb-8">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2.5">
          <span className="flex h-9 w-9 items-center justify-center rounded-xl border border-teyvat-line-strong bg-teyvat-gold/10 text-teyvat-gold">
            <UserRound className="h-4 w-4" />
          </span>
          <h2 className="text-[22px] leading-tight font-semibold tracking-[0.01em] text-teyvat-gold">
            {t('demo.accountTitle')}
          </h2>
        </div>
      </div>

      {/* 演示提示条：措辞由服务端从库里给（可运营），拿不到才用词典兜底 ✓ */}
      <Alert tone="info" className="tx-enter">
        <span className="flex gap-2">
          <Info className="mt-0.5 h-3.5 w-3.5 shrink-0 text-teyvat-gold" />
          <span className="text-[11px]">{data.notice || t('demo.noticeFallback')}</span>
        </span>
      </Alert>

      <PlayerProfileView profile={profile} selectedId={selectedId} onSelect={setSelectedId} />

      <div className="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-teyvat-line bg-teyvat-panel p-4">
        <p className="text-xs leading-relaxed text-teyvat-text-3">{t('demo.accountCtaHint')}</p>
        <Link to="/login" className={buttonClass({ variant: 'primary' })}>
          {t('demo.loginCta')}
        </Link>
      </div>

      {/* 「派蒙的分析」：与真实页同一个悬浮按钮 + 同一个浮动面板 ✓（内容是库里预置的 ✓） */}
      {selected && selected.analysis && (
        <DemoAnalysis
          character={selected.card}
          analysis={selected.analysis}
          charKey={selected.charKey}
        />
      )}
    </div>
  )
}

/** 演示档案加载骨架：形状与真实档案一致（头部 + 角色条 + 属性格） */
function DemoProfileSkeleton() {
  return (
    <div className="mx-auto max-w-5xl space-y-6 px-4 pt-6 sm:px-6 sm:pt-8">
      <Skeleton className="h-6 w-32" />
      <Skeleton className="h-14 rounded-xl" />
      <div className="rounded-2xl border border-teyvat-line bg-teyvat-panel p-3 shadow-panel sm:p-4">
        <div className="flex items-center gap-3">
          <Skeleton className="h-10 w-10 rounded-xl" />
          <Skeleton className="h-4 w-36" />
          <Skeleton className="ml-auto h-6 w-20 rounded-full" />
        </div>
      </div>
      <div className="flex gap-2">
        {[0, 1, 2, 3].map((i) => (
          <Skeleton key={i} className="h-12 w-12 rounded-lg" />
        ))}
      </div>
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
        {[0, 1, 2, 3, 4, 5, 6, 7].map((i) => (
          <Skeleton key={i} className="h-9 rounded-xl" />
        ))}
      </div>
    </div>
  )
}
