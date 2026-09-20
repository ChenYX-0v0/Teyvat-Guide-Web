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

/**
 * 圣遗物卡片（响应式紧凑版）。
 *
 * 背景：移动端此前是**单列 + 副词条全展开**，5 件约 950px —— 比一屏还高，得来回翻。
 * 现在：手机上两列紧凑排布（部位/等级、主词条、件名、套装、副词条——**一项不藏**），
 * 约 3 行、460px 内即可看全 5 件；`sm` 以上恢复宽松内边距，`lg` 一行 5 件
 * （与游戏内圣遗物界面的观感一致）。
 *
 * 取舍：只在**字号/间距/列数**上做收敛，不隐藏任何数据（"完整配置"是目标）。
 */
export default function RelicDetailCard({ relic }: { relic: RelicInfo }) {
  const rarity = relic.rarity && relic.rarity >= 3 ? relic.rarity : 5

  return (
    <div
      className={clsx(
        'rounded-2xl border bg-teyvat-panel p-2.5 shadow-panel sm:p-4',
        rarityBorder[rarity] ?? rarityBorder[5],
      )}
    >
      <div className="flex items-center gap-1.5">
        {/* 部位图标（参考提瓦特小助手）：旧快照没有该字段，缺失时整块不渲染 */}
        {relic.iconUrl && (
          <img
            src={relic.iconUrl}
            alt={relicPosNames[relic.pos] ?? relic.pos}
            loading="lazy"
            className="h-6 w-6 shrink-0 rounded-md border border-teyvat-line bg-teyvat-raised object-cover sm:h-8 sm:w-8"
          />
        )}
        <span className="rounded-md border border-teyvat-line bg-white/[0.03] px-1.5 py-0.5 text-[11px] leading-tight text-teyvat-text-2 sm:px-2 sm:text-xs">
          {relicPosNames[relic.pos] ?? relic.pos}
        </span>
        <span className="ml-auto shrink-0 text-[11px] leading-tight text-teyvat-text-3 sm:text-xs">
          +{relic.level}
        </span>
      </div>

      {/* 主词条：看"配置"时最该一眼扫到的信息（参考提瓦特小助手：主词条最醒目） */}
      {relic.mainStat && (
        <p
          className={clsx(
            'mt-1.5 truncate text-sm leading-tight font-semibold sm:mt-2',
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

      {/* 套装：移动端只留套装（配置里最关键的归属信息，窄卡片也只够一行）；
          sm 以上空间充足，恢复"件名 + 套装" */}
      <p
        className={clsx(
          'mt-0.5 truncate text-[11px] leading-tight sm:hidden',
          rarityText[rarity] ?? rarityText[5],
        )}
      >
        {relic.set || relic.name}
      </p>
      <p
        className={clsx(
          'mt-0.5 hidden truncate text-xs leading-tight sm:block',
          rarityText[rarity] ?? rarityText[5],
        )}
      >
        {relic.name}
      </p>
      {relic.set && relic.set !== relic.name && (
        <p className="hidden truncate text-[11px] leading-tight text-teyvat-text-3 sm:block sm:text-xs">
          {relic.set}
        </p>
      )}

      {relic.subStats && relic.subStats.length > 0 && (
        <div className="mt-1.5 space-y-0.5 border-t border-teyvat-line pt-1.5 sm:mt-2 sm:pt-2">
          {relic.subStats.map((s) => (
            <p
              key={s.name}
              className={clsx(
                'truncate text-[11px] leading-tight sm:text-xs',
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
                    'ml-1 inline-flex h-4 w-4 items-center justify-center rounded-full border align-middle text-[10px] leading-none font-medium',
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
