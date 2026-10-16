import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { Info, UserRound } from 'lucide-react'
import PlayerProfileView from '@/components/account/PlayerProfileView'
import Card from '@/components/common/Card'
import Chip from '@/components/common/Chip'
import { demoApi } from '@/api/demo'
import { buttonClass } from '@/utils/buttonStyles'
import { elementColor } from '@/utils/elementColor'
import { renderAnswer } from '@/utils/renderAnswer'
import { t } from '@/i18n'
import type { DemoShowcase } from '@/types/demo'
import type { PlayerProfile } from '@/types/genshin'

/**
 * 演示档案（2026-10-17，公开路由 `/demo`）。
 *
 * 数据来自 `GET /api/demo/showcase`：一个**模拟旅行者账号头** + 4 位演示角色（各自的角色卡 + 预置「分析」）✓。
 *
 * 复用口径：演示角色的 `card` 与真实档案**同构** ✓ ⇒ 整个档案视图（角色条 / 面板 /
 * 圣遗物卡片 / 评分与套装统计）**一行都不用重写**，直接交给 `PlayerProfileView` ✓。
 * 这也是刻意不建"演示专用组件"的原因 ✗：两套组件必然漂移，日后真实档案改了演示页就说不一致 ✗。
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
    return <p className="py-16 text-center text-sm text-teyvat-text-3">{t('common.loading')}</p>
  }
  if (error) {
    return <p className="py-16 text-center text-sm text-err">{error}</p>
  }
  if (!profile || !data) {
    return <p className="py-16 text-center text-sm text-teyvat-text-3">{t('demo.empty')}</p>
  }

  return (
    <div className="mx-auto max-w-5xl space-y-6 px-4 pt-6 pb-10 sm:px-6">
      <div className="flex flex-wrap items-center gap-3">
        <h2 className="flex items-center gap-2 text-xl font-semibold text-teyvat-gold">
          <UserRound className="h-5 w-5" />
          {t('demo.accountTitle')}
        </h2>
      </div>

      {/* 演示提示语：措辞由服务端从库里给（可运营），拿不到才用词典兜底 ✓ */}
      <div className="flex gap-2 rounded-2xl border border-teyvat-line-strong bg-teyvat-gold/5 p-4">
        <Info className="mt-0.5 h-3.5 w-3.5 shrink-0 text-teyvat-gold" />
        <p className="text-[11px] leading-relaxed text-teyvat-text-2">
          {data.notice || t('demo.noticeFallback')}
        </p>
      </div>

      <PlayerProfileView profile={profile} selectedId={selectedId} onSelect={setSelectedId} />

      {/* 预置「分析」：真实页这里是派蒙现场体检（要额度），演示页直接把库里那份念出来 ✓ */}
      {selected && selected.analysis && (
        <Card className="p-5">
          <div className="flex flex-wrap items-center gap-2">
            <h3 className="text-sm font-medium text-teyvat-text">
              {t('demo.analysisTitle', { name: selected.name })}
            </h3>
            <Chip dot={elementColor(selected.element)}>{selected.role}</Chip>
            <span className="text-[11px] text-teyvat-text-4">{t('demo.analysisNote')}</span>
          </div>
          <div className="mt-3 text-sm leading-relaxed whitespace-pre-wrap text-teyvat-text-2">
            {renderAnswer(selected.analysis)}
          </div>
        </Card>
      )}

      <div className="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-teyvat-line bg-teyvat-panel p-4">
        <p className="text-xs leading-relaxed text-teyvat-text-3">{t('demo.accountCtaHint')}</p>
        <Link to="/login" className={buttonClass({ variant: 'primary' })}>
          {t('demo.loginCta')}
        </Link>
      </div>
    </div>
  )
}
