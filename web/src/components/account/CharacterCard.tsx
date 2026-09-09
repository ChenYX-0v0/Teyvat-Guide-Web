import { useState } from 'react'
import clsx from 'clsx'
import { ChevronDown, Star } from 'lucide-react'
import { t } from '@/i18n'
import { relicPosNames, type CharacterCard as CharacterCardType } from '@/types/genshin'

const elementStyle: Record<string, string> = {
  风: 'bg-teal-500/20 text-teal-300',
  岩: 'bg-amber-500/20 text-amber-300',
  雷: 'bg-purple-500/20 text-purple-300',
  草: 'bg-lime-500/20 text-lime-300',
  水: 'bg-blue-500/20 text-blue-300',
  火: 'bg-red-500/20 text-red-300',
  冰: 'bg-cyan-500/20 text-cyan-300',
}

export default function CharacterCard({ c }: { c: CharacterCardType }) {
  const [expanded, setExpanded] = useState(false)
  const isFive = c.rarity >= 5
  const hasRelics = !!c.relics?.length

  return (
    <div
      className={clsx(
        'overflow-hidden rounded-2xl border bg-teyvat-panel/60 transition',
        isFive ? 'border-teyvat-gold/30' : 'border-white/10',
        hasRelics && 'cursor-pointer hover:border-teyvat-gold/40',
      )}
      onClick={() => hasRelics && setExpanded(!expanded)}
    >
      <div className="relative aspect-square bg-white/5">
        {c.iconUrl ? (
          <img
            src={c.iconUrl}
            alt={c.name}
            loading="lazy"
            className="h-full w-full object-cover"
          />
        ) : (
          <div className="flex h-full items-center justify-center text-2xl font-semibold text-white/30">
            {c.name[0]}
          </div>
        )}
        {c.element && (
          <span
            className={clsx(
              'absolute left-2 top-2 rounded-md px-1.5 py-0.5 text-xs',
              elementStyle[c.element] ?? 'bg-white/10 text-white/70',
            )}
          >
            {c.element}
          </span>
        )}
        <span className="absolute bottom-2 right-2 rounded-md bg-black/55 px-1.5 py-0.5 text-xs font-medium">
          Lv.{c.level}
        </span>
        {c.constellation > 0 && (
          <span className="absolute bottom-2 left-2 rounded-md bg-black/55 px-1.5 py-0.5 text-xs text-teyvat-gold">
            {t('character.constellation', { n: c.constellation })}
          </span>
        )}
      </div>

      <div className="space-y-1.5 p-3">
        <div className="flex items-center justify-between gap-1">
          <span className="truncate text-sm font-medium">{c.name}</span>
          <span className="flex shrink-0 items-center gap-0.5 text-xs text-teyvat-gold">
            <Star className="h-3 w-3 fill-teyvat-gold" />
            {c.rarity || '—'}
          </span>
        </div>
        <p className="text-xs text-white/55">{t('character.fetter', { n: c.fetter })}</p>
        {c.weapon && (
          <p className="truncate text-xs text-white/55">
            {c.weapon.name} · Lv.{c.weapon.level}
            {c.weapon.affix > 0 && ` · ${t('character.refine', { n: c.weapon.affix })}`}
          </p>
        )}
        {hasRelics && (
          <p className="flex items-center justify-between text-xs text-white/40">
            <span>{t('character.relicsCount', { n: c.relics!.length })}</span>
            <ChevronDown
              className={clsx('h-3.5 w-3.5 transition-transform', expanded && 'rotate-180')}
            />
          </p>
        )}

        {/* 展开的角色面板 + 圣遗物详情 */}
        {expanded && hasRelics && (
          <div
            className="space-y-2 border-t border-white/10 pt-2.5"
            onClick={(e) => e.stopPropagation()}
          >
            {/* 面板属性 */}
            {c.stats && c.stats.length > 0 && (
              <div className="grid grid-cols-2 gap-1.5">
                {c.stats.map((s) => (
                  <div
                    key={s.name}
                    className="flex items-center justify-between rounded-lg bg-teyvat-gold/10 px-2 py-1.5"
                  >
                    <span className="text-xs text-white/60">{s.name}</span>
                    <span className="text-xs font-medium text-teyvat-gold">{s.value}</span>
                  </div>
                ))}
              </div>
            )}

            {c.relics!.map((r) => (
              <div key={r.pos} className="rounded-xl bg-white/5 p-2.5">
                <div className="flex items-center justify-between">
                  <span className="text-xs font-medium text-white/75">
                    {relicPosNames[r.pos] ?? r.pos}
                  </span>
                  <span className="text-xs text-white/40">+{r.level}</span>
                </div>
                <p className="mt-1 truncate text-xs text-teyvat-gold">
                  {r.set || r.name}
                </p>
                {r.mainStat && <p className="text-xs text-white/70">{r.mainStat}</p>}
                {r.subStats && r.subStats.length > 0 && (
                  <div className="mt-1 space-y-0.5">
                    {r.subStats.map((s) => (
                      <p key={s.name} className="text-xs text-white/45">
                        {s.name} +{s.value}
                        {s.count > 1 && (
                          <span className="text-teyvat-gold">
                            {' '}
                            {t('character.enhanceTimes', { n: s.count - 1 })}
                          </span>
                        )}
                      </p>
                    ))}
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
