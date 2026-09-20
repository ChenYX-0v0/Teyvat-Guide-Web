import clsx from 'clsx'
import { t } from '@/i18n'
import { relicPosNames, type RelicInfo } from '@/types/genshin'

/** 稀有度：边框与件名配色（原神约定 5★金 / 4★紫 / 3★蓝） */
const rarityBorder: Record<number, string> = {
  5: 'border-rarity-5/45',
  4: 'border-rarity-4/45',
  3: 'border-rarity-3/40',
}
const rarityText: Record<number, string> = {
  5: 'text-teyvat-gold',
  4: 'text-rarity-4',
  3: 'text-rarity-3',
}

export default function RelicDetailCard({ relic }: { relic: RelicInfo }) {
  const rarity = relic.rarity && relic.rarity >= 3 ? relic.rarity : 5

  return (
    <div
      className={clsx(
        'rounded-2xl border bg-teyvat-panel p-4 shadow-panel',
        rarityBorder[rarity] ?? rarityBorder[5],
      )}
    >
      <div className="flex items-center justify-between gap-2">
        <span className="rounded-md border border-teyvat-line bg-white/[0.03] px-2 py-0.5 text-xs text-teyvat-text-2">
          {relicPosNames[relic.pos] ?? relic.pos}
        </span>
        <span className="text-xs text-teyvat-text-3">+{relic.level}</span>
      </div>

      {/* 件名（缺）时用套装名 */}
      <p className={clsx('mt-2 truncate text-sm font-medium', rarityText[rarity] ?? rarityText[5])}>
        {relic.name}
      </p>
      {relic.set && relic.set !== relic.name && (
        <p className="truncate text-xs text-teyvat-text-3">{relic.set}</p>
      )}

      {/* 主词条：主要属性金色 / 次要暖白 / 其他弱化 */}
      {relic.mainStat && (
        <p
          className={clsx(
            'mt-2 text-xs font-medium',
            relic.mainStatTier === 'primary'
              ? 'text-teyvat-gold'
              : relic.mainStatTier === 'secondary'
                ? 'text-teyvat-text'
                : 'text-teyvat-text-3',
          )}
        >
          {relic.mainStat}
        </p>
      )}

      {relic.subStats && relic.subStats.length > 0 && (
        <div className="mt-2 space-y-0.5 border-t border-teyvat-line pt-2">
          {relic.subStats.map((s) => (
            <p
              key={s.name}
              className={clsx(
                'text-xs',
                s.tier === 'primary'
                  ? 'text-teyvat-gold'
                  : s.tier === 'secondary'
                    ? 'text-teyvat-text-2'
                    : 'text-teyvat-text-3',
              )}
            >
              {s.name} +{s.value}
              {/* 强化次数圆圈徽标：仅主/次属性显示，颜色随词条分级 */}
              {s.count > 1 && (s.tier === 'primary' || s.tier === 'secondary') && (
                <span
                  className={clsx(
                    'ml-1.5 inline-flex h-4 w-4 items-center justify-center rounded-full border align-middle text-[10px] leading-none font-medium',
                    s.tier === 'primary'
                      ? 'border-teyvat-line-strong bg-teyvat-gold/15 text-teyvat-gold'
                      : 'border-teyvat-line bg-white/[0.04] text-teyvat-text-2',
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
