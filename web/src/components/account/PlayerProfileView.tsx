import { useState } from 'react'
import clsx from 'clsx'
import RelicDetailCard from './RelicDetailCard'
import Card from '@/components/common/Card'
import Chip from '@/components/common/Chip'
import { elementColor } from '@/utils/elementColor'
import { t } from '@/i18n'
import type { PlayerProfile } from '@/types/genshin'

export default function PlayerProfileView({ profile }: { profile: PlayerProfile }) {
  // 选中角色：重新查询后回落到第一个
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const selected =
    profile.characters.find((c) => c.id === selectedId) ?? profile.characters[0]

  const stats = [
    { label: t('profile.statsAchievements'), value: profile.achievements },
    { label: t('profile.statsWorldLevel'), value: profile.worldLevel },
    { label: t('profile.statsAvatars'), value: profile.characters.length },
  ]

  // ── 圣遗物一览（参考"提瓦特小助手"的套装 / 词条统计）──
  // 套装构成：按出现顺序分组，如 黄金剧团 ×4 · 昔日宗室之仪 ×1
  const relicSets: { name: string; count: number }[] = []
  for (const r of selected?.relics ?? []) {
    const key = r.set || r.name
    const hit = relicSets.find((g) => g.name === key)
    if (hit) hit.count += 1
    else relicSets.push({ name: key, count: 1 })
  }
  // 有效词条：只统计"命中角色推荐词条（statWeights）"的词条强化次数（含初始获得，即"条"数），
  // 也就是这套圣遗物有多少条词条加在了对的属性上 —— 数值越大说明副词条越不歪
  const rollsByStat = (selected?.statWeights?.subStats ?? [])
    .map((w) => ({
      name: w.name,
      count: (selected?.relics ?? []).reduce(
        (sum, r) =>
          sum + (r.subStats ?? []).filter((s) => s.name === w.name).reduce((a, s) => a + s.count, 0),
        0,
      ),
    }))
    .filter((x) => x.count > 0)
    .sort((a, b) => b.count - a.count)
  const totalRolls = rollsByStat.reduce((a, x) => a + x.count, 0)

  return (
    <div className="space-y-6">
      {/* 玩家头部 */}
      <Card className="flex flex-wrap items-center gap-5 p-5">
        {profile.avatarUrl ? (
          <img
            src={profile.avatarUrl}
            alt={profile.nickname}
            className="h-16 w-16 rounded-2xl border border-teyvat-line-strong object-cover"
          />
        ) : (
          <div className="flex h-16 w-16 items-center justify-center rounded-2xl border border-teyvat-line-strong bg-teyvat-gold/10 text-xl font-semibold text-teyvat-gold">
            {profile.nickname[0] ?? '?'}
          </div>
        )}
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <h2 className="text-lg font-semibold text-teyvat-text">{profile.nickname}</h2>
            <Chip tone="gold">Lv.{profile.level}</Chip>
          </div>
          <p className="mt-1 text-xs text-teyvat-text-3">UID {profile.uid}</p>
          <p className="mt-1 max-w-md truncate text-xs text-teyvat-text-4">
            {profile.signature || t('profile.noSignature')}
          </p>
        </div>
        <div className="ml-auto grid grid-cols-3 gap-3">
          {stats.map((s) => (
            <div key={s.label} className="rounded-xl bg-teyvat-raised px-4 py-2 text-center">
              <p className="text-base font-semibold text-teyvat-text">{s.value}</p>
              <p className="mt-0.5 text-xs text-teyvat-text-3">{s.label}</p>
            </div>
          ))}
        </div>
      </Card>

      {/* 角色选择条 + 选中角色详情（主从布局） */}
      {profile.characters.length > 0 ? (
        <div className="space-y-5">
          <div>
            <h3 className="mb-2 text-sm font-medium text-teyvat-text">
              {t('profile.charactersCount', { count: profile.characters.length })}
            </h3>
            <div className="flex gap-2 overflow-x-auto pb-2">
              {profile.characters.map((c) => {
                const accent = elementColor(c.element)
                return (
                  <button
                    key={c.id}
                    onClick={() => setSelectedId(c.id)}
                    className={clsx(
                      'w-20 shrink-0 rounded-xl border p-2 text-center transition duration-150 focus-visible:shadow-glow focus-visible:outline-none',
                      selected?.id === c.id
                        ? 'border-teyvat-line-strong bg-teyvat-raised'
                        : 'border-teyvat-line bg-teyvat-panel hover:border-teyvat-line-strong',
                    )}
                  >
                    <span className="relative mx-auto block h-12 w-12">
                      {c.iconUrl ? (
                        <img
                          src={c.iconUrl}
                          alt={c.name}
                          loading="lazy"
                          className="h-12 w-12 rounded-lg object-cover"
                        />
                      ) : (
                        <span className="flex h-12 w-12 items-center justify-center rounded-lg bg-teyvat-raised text-lg font-semibold text-teyvat-text-3">
                          {c.name[0]}
                        </span>
                      )}
                      {/* 元素徽记：小面积点亮（视觉方案 §6.4） */}
                      {accent && (
                        <span
                          className="absolute -right-1 -bottom-1 h-3.5 w-3.5 rounded-full border border-teyvat-ink"
                          style={{ background: accent }}
                          title={c.element}
                        />
                      )}
                    </span>
                    <p className="mt-1 truncate text-xs text-teyvat-text-2">{c.name}</p>
                    <p className="text-xs text-teyvat-text-3">Lv.{c.level}</p>
                  </button>
                )
              })}
            </div>
          </div>

          {/* 选中角色详情 */}
          {selected && (
            <div className="space-y-5">
              {/* 角色概要 */}
              <div className="flex flex-wrap items-center gap-x-6 gap-y-2 rounded-2xl border border-teyvat-line-strong bg-teyvat-gold/5 px-5 py-4">
                <div className="flex items-center gap-2">
                  <p className="text-sm font-semibold text-teyvat-text">{selected.name}</p>
                  <Chip dot={elementColor(selected.element)}>{selected.element}</Chip>
                </div>
                <p className="text-xs text-teyvat-text-2">
                  Lv.{selected.level} ·{' '}
                  {t('character.constellation', { n: selected.constellation })}
                </p>
                <p className="text-xs text-teyvat-text-2">
                  {t('character.fetter', { n: selected.fetter })}
                </p>
                {selected.weapon && (
                  <p className="text-xs text-teyvat-text-2">
                    {t('character.weaponLabel')}: {selected.weapon.name} · Lv.
                    {selected.weapon.level}
                    {selected.weapon.affix > 0 &&
                      ` · ${t('character.refine', { n: selected.weapon.affix })}`}
                  </p>
                )}
              </div>

              {/* 面板属性 */}
              {selected.stats && selected.stats.length > 0 && (
                <div>
                  <h4 className="mb-2 text-sm font-medium text-teyvat-text">
                    {t('profile.statsTitle')}
                  </h4>
                  <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
                    {selected.stats.map((s) => {
                      // 主/次属性金色，未分级弱化
                      const highlighted = s.tier === 'primary' || s.tier === 'secondary'
                      return (
                        <div
                          key={s.name}
                          className={clsx(
                            'flex items-center justify-between rounded-xl border px-3 py-2',
                            highlighted
                              ? 'border-teyvat-line-strong bg-teyvat-gold/5'
                              : 'border-teyvat-line bg-teyvat-panel',
                          )}
                        >
                          <span
                            className={clsx(
                              'text-xs',
                              highlighted ? 'text-teyvat-gold/85' : 'text-teyvat-text-4',
                            )}
                          >
                            {s.name}
                          </span>
                          <span
                            className={clsx(
                              'text-sm font-semibold',
                              highlighted ? 'text-teyvat-gold' : 'text-teyvat-text-3',
                            )}
                          >
                            {s.value}
                          </span>
                        </div>
                      )
                    })}
                  </div>
                </div>
              )}

              {/* 圣遗物详情 */}
              {selected.relics && selected.relics.length > 0 && (
                <div>
                  <h4 className="mb-1.5 text-sm font-medium text-teyvat-text">
                    {t('character.relicsCount', { n: selected.relics.length })}
                  </h4>

                  {/* 一览：套装构成 + 有效词条（参考提瓦特小助手，让"配置"第一眼就读得到） */}
                  <div className="mb-2 space-y-1.5">
                    <div className="flex flex-wrap items-center gap-1.5">
                      {relicSets.map((g) => (
                        // 满 4 件用金色：4 件套是该套装的生效阈值
                        <Chip key={g.name} tone={g.count >= 4 ? 'gold' : undefined}>
                          {g.name} ×{g.count}
                        </Chip>
                      ))}
                    </div>
                    {totalRolls > 0 && (
                      <div className="flex flex-wrap items-center gap-1.5">
                        <span className="text-[11px] text-teyvat-text-4">
                          {t('character.effectiveRolls', { n: totalRolls })}
                        </span>
                        {rollsByStat.slice(0, 4).map((x) => (
                          <span
                            key={x.name}
                            className="rounded-full border border-teyvat-line bg-white/[0.03] px-2 py-0.5 text-[11px] leading-tight text-teyvat-text-2"
                          >
                            {x.name} {x.count}
                          </span>
                        ))}
                      </div>
                    )}
                  </div>

                  {/* 移动端两列（5 件约 3 行、一屏看全）；lg 一行 5 件，与游戏内圣遗物界面观感一致 */}
                  <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 sm:gap-3 lg:grid-cols-5">
                    {selected.relics.map((r) => (
                      <RelicDetailCard key={r.pos} relic={r} />
                    ))}
                  </div>
                </div>
              )}
            </div>
          )}
        </div>
      ) : (
        <Card className="p-8 text-center text-sm text-teyvat-text-3">
          {t('account.emptyShowcase')}
        </Card>
      )}
    </div>
  )
}
