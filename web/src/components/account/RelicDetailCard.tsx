import clsx from 'clsx'
import { t } from '@/i18n'
import { relicPosNames, type RelicInfo } from '@/types/genshin'

export default function RelicDetailCard({ relic }: { relic: RelicInfo }) {
  return (
    <div className="rounded-2xl border border-white/10 bg-teyvat-panel/60 p-4">
      <div className="flex items-center justify-between gap-2">
        <span className="rounded-md bg-white/10 px-2 py-0.5 text-xs text-white/70">
          {relicPosNames[relic.pos] ?? relic.pos}
        </span>
        <span className="text-xs text-white/40">+{relic.level}</span>
      </div>

      {/* 件名（缺）时用套装名 */}
      <p className="mt-2 truncate text-sm font-medium text-teyvat-gold">
        {relic.name}
      </p>
      {relic.set && relic.set !== relic.name && (
        <p className="truncate text-xs text-white/40">{relic.set}</p>
      )}

      {/* 主词条：主要属性金色 / 次要白色 / 其他灰色 */}
      {relic.mainStat && (
        <p
          className={clsx(
            'mt-2 text-xs font-medium',
            relic.mainStatTier === 'primary'
              ? 'text-teyvat-gold'
              : relic.mainStatTier === 'secondary'
                ? 'text-white'
                : 'text-white/50',
          )}
        >
          {relic.mainStat}
        </p>
      )}

      {relic.subStats && relic.subStats.length > 0 && (
        <div className="mt-2 space-y-0.5 border-t border-white/10 pt-2">
          {relic.subStats.map((s) => (
            <p
              key={s.name}
              className={clsx(
                'text-xs',
                s.tier === 'primary'
                  ? 'text-teyvat-gold'
                  : s.tier === 'secondary'
                    ? 'text-white/90'
                    : 'text-white/40',
              )}
            >
              {s.name} +{s.value}
              {/* 强化次数圆圈徽标：仅主/次属性显示，颜色随词条分级 */}
              {s.count > 1 && (s.tier === 'primary' || s.tier === 'secondary') && (
                <span
                  className={clsx(
                    'ml-1.5 inline-flex h-4 w-4 items-center justify-center rounded-full border align-middle text-[10px] font-medium leading-none',
                    s.tier === 'primary'
                      ? 'border-teyvat-gold/60 bg-teyvat-gold/15 text-teyvat-gold'
                      : 'border-white/50 bg-white/10 text-white',
                  )}
                  title={t('character.enhanceTimes', { n: s.count - 1 })}
                >
                  {s.count - 1}
                </span>
              )}
            </p>
          ))}
        </div>
      )}
    </div>
  )
}
