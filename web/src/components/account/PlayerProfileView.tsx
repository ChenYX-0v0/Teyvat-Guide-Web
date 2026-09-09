import { AlertTriangle } from 'lucide-react'
import { useState } from 'react'
import clsx from 'clsx'
import RelicDetailCard from './RelicDetailCard'
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

  return (
    <div className="space-y-6">
      {profile.warnings && profile.warnings.length > 0 && (
        <div className="rounded-2xl border border-amber-400/30 bg-amber-400/10 p-4 text-xs text-amber-200">
          <p className="flex items-center gap-1.5 font-medium">
            <AlertTriangle className="h-3.5 w-3.5" />
            {t('profile.partialFailed')}
          </p>
          <ul className="mt-1.5 list-inside list-disc space-y-0.5 text-amber-200/80">
            {profile.warnings.map((w) => (
              <li key={w}>{w}</li>
            ))}
          </ul>
        </div>
      )}

      {/* 玩家头部 */}
      <div className="flex flex-wrap items-center gap-5 rounded-2xl border border-white/10 bg-teyvat-panel/60 p-5">
        {profile.avatarUrl ? (
          <img
            src={profile.avatarUrl}
            alt={profile.nickname}
            className="h-16 w-16 rounded-2xl border border-teyvat-gold/30 object-cover"
          />
        ) : (
          <div className="flex h-16 w-16 items-center justify-center rounded-2xl border border-teyvat-gold/30 bg-teyvat-gold/10 text-xl font-semibold text-teyvat-gold">
            {profile.nickname[0] ?? '?'}
          </div>
        )}
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <h2 className="text-lg font-semibold text-white">{profile.nickname}</h2>
            <span className="rounded-md bg-teyvat-gold/15 px-2 py-0.5 text-xs text-teyvat-gold">
              Lv.{profile.level}
            </span>
          </div>
          <p className="mt-1 text-xs text-white/45">UID {profile.uid}</p>
          <p className="mt-1 max-w-md truncate text-xs text-white/40">
            {profile.signature || t('profile.noSignature')}
          </p>
        </div>
        <div className="ml-auto grid grid-cols-3 gap-3">
          {stats.map((s) => (
            <div key={s.label} className="rounded-xl bg-white/5 px-4 py-2 text-center">
              <p className="text-base font-semibold text-white">{s.value}</p>
              <p className="mt-0.5 text-xs text-white/45">{s.label}</p>
            </div>
          ))}
        </div>
      </div>

      {/* 角色选择条 + 选中角色详情（主从布局） */}
      {profile.characters.length > 0 ? (
        <div className="space-y-5">
          <div>
            <h3 className="mb-2 text-sm font-medium text-white">
              {t('profile.charactersCount', { count: profile.characters.length })}
            </h3>
            <div className="flex gap-2 overflow-x-auto pb-2">
              {profile.characters.map((c) => (
                <button
                  key={c.id}
                  onClick={() => setSelectedId(c.id)}
                  className={clsx(
                    'w-20 shrink-0 rounded-xl border p-2 text-center transition',
                    selected?.id === c.id
                      ? 'border-teyvat-gold bg-teyvat-gold/10'
                      : 'border-white/10 bg-teyvat-panel/60 hover:border-white/30',
                  )}
                >
                  {c.iconUrl ? (
                    <img
                      src={c.iconUrl}
                      alt={c.name}
                      loading="lazy"
                      className="mx-auto h-12 w-12 rounded-lg object-cover"
                    />
                  ) : (
                    <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-lg bg-white/10 text-lg font-semibold text-white/40">
                      {c.name[0]}
                    </div>
                  )}
                  <p className="mt-1 truncate text-xs text-white/70">{c.name}</p>
                  <p className="text-xs text-white/40">Lv.{c.level}</p>
                </button>
              ))}
            </div>
          </div>

          {/* 选中角色详情 */}
          {selected && (
            <div className="space-y-5">
              {/* 角色概要 */}
              <div className="flex flex-wrap items-center gap-x-6 gap-y-2 rounded-2xl border border-teyvat-gold/20 bg-teyvat-gold/5 px-5 py-4">
                <div>
                  <p className="text-sm font-semibold text-white">{selected.name}</p>
                  <p className="text-xs text-white/45">{selected.element}</p>
                </div>
                <p className="text-xs text-white/60">
                  Lv.{selected.level} ·{' '}
                  {t('character.constellation', { n: selected.constellation })}
                </p>
                <p className="text-xs text-white/60">
                  {t('character.fetter', { n: selected.fetter })}
                </p>
                {selected.weapon && (
                  <p className="text-xs text-white/60">
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
                  <h4 className="mb-2 text-sm font-medium text-white">
                    {t('profile.statsTitle')}
                  </h4>
                  <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
                    {selected.stats.map((s) => (
                      <div
                        key={s.name}
                        className="flex items-center justify-between rounded-xl border border-white/10 bg-teyvat-panel/60 px-3 py-2"
                      >
                        <span className="text-xs text-white/55">{s.name}</span>
                        <span className="text-sm font-semibold text-teyvat-gold">
                          {s.value}
                        </span>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {/* 圣遗物详情 */}
              {selected.relics && selected.relics.length > 0 && (
                <div>
                  <h4 className="mb-2 text-sm font-medium text-white">
                    {t('character.relicsCount', { n: selected.relics.length })}
                  </h4>
                  <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
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
        <div className="rounded-2xl border border-white/10 bg-teyvat-panel/60 p-8 text-center text-sm text-white/40">
          {t('account.emptyShowcase')}
        </div>
      )}
    </div>
  )
}
