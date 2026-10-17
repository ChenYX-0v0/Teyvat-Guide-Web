import { useState } from 'react'
import clsx from 'clsx'
import { UserRound } from 'lucide-react'
import RelicDetailCard from './RelicDetailCard'
import Card from '@/components/common/Card'
import Chip from '@/components/common/Chip'
import EmptyState from '@/components/common/EmptyState'
import { elementColor } from '@/utils/elementColor'
import { t } from '@/i18n'
import type { CollectedCharacter } from '@/api/genshin'
import type { CharacterCard, PlayerProfile } from '@/types/genshin'

interface Props {
  profile: PlayerProfile
  /**
   * 已收录角色池（2026-10-11，**只增/只改不删** ✓，见 `web/04` §4.1 与 `model.PlayerCharacter`）。
   *
   * 传了且非空 ⇒ 角色条渲染**累积池** ✓ —— 用户轮换展柜（每轮最多 12 位）导入进来的角色不会消失 ✓；
   * 没传 / 为空（列表还没拉到、或接口失败）⇒ 退回只显示**当前展柜** ✓（不影响主流程 ✓）。
   */
  collected?: CollectedCharacter[]
  /** 受控选中角色：由 AccountPage 持有，悬浮体检面板据此知道"当前在看谁"（设计 §7.5） */
  selectedId: number | null
  onSelect: (id: number) => void
}

/** 是不是"累积池里的角色"（只有它带 inShowcase / lastSeenAt ✓） */
function isCollected(c: CharacterCard | CollectedCharacter): c is CollectedCharacter {
  return 'lastSeenAt' in c
}

export default function PlayerProfileView({ profile, collected, selectedId, onSelect }: Props) {
  // 角色条用哪个集合：优先**累积池** ✓，拿不到才退回当前展柜 ✓（两个口径别混 ✗）
  const pool: (CharacterCard | CollectedCharacter)[] =
    collected && collected.length > 0 ? collected : profile.characters
  // 选中角色：重新查询后回落到第一个（旧 id 不在集合里时自动兜底）
  const selected = pool.find((c) => c.id === selectedId) ?? pool[0]
  // 玩家头部默认折叠：只留「头像 + 昵称 + 等级 + UID」，签名与三个统计点开才看（把首屏留给展柜）
  const [headOpen, setHeadOpen] = useState(false)

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
      {/* 玩家头部：默认折叠成一行（头像 + 昵称 + 等级 + UID），点开才看签名与三个统计 —— 
          手机上把首屏留给展柜与圣遗物 */}
      <Card className="p-3 sm:p-4">
        <button
          type="button"
          onClick={() => setHeadOpen((v) => !v)}
          aria-expanded={headOpen}
          aria-label={headOpen ? t('common.collapse') : t('common.expand')}
          className="flex w-full items-center gap-3 text-left focus-visible:shadow-glow focus-visible:outline-none"
        >
          {profile.avatarUrl ? (
            <img
              src={profile.avatarUrl}
              alt={profile.nickname}
              width={40}
              height={40}
              className="h-10 w-10 shrink-0 rounded-xl border border-teyvat-line-strong object-cover"
            />
          ) : (
            <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl border border-teyvat-line-strong bg-teyvat-gold/10 text-base font-semibold text-teyvat-gold">
              {profile.nickname[0] ?? '?'}
            </span>
          )}
          <span className="flex min-w-0 items-center gap-2">
            <span className="truncate text-[17px] leading-tight font-semibold tracking-[0.01em] text-teyvat-text">
              {profile.nickname}
            </span>
            <Chip tone="gold" className="tabular-nums">
              Lv.{profile.level}
            </Chip>
          </span>
          <span className="ml-auto flex shrink-0 items-center gap-2">
            {/* uid 为空则不渲染（2026-10-17）：演示档案是编的、没有游戏 UID，
                空值渲染出"UID "这样的半截文案很难看 ✗（真实档案一定有 uid ✓，不受影响 ✓） */}
            {profile.uid && (
              <span className="text-[11px] tracking-[0.04em] text-teyvat-text-4 tabular-nums">
                UID {profile.uid}
              </span>
            )}
            <span
              className={clsx('block text-[10px] leading-none text-teyvat-text-4', headOpen && 'rotate-180')}
            >
              ▾
            </span>
          </span>
        </button>

        {headOpen && (
          <div className="mt-3 space-y-3 border-t border-teyvat-line pt-3">
            <p className="truncate text-xs text-teyvat-text-4">
              {profile.signature || t('profile.noSignature')}
            </p>
            <div className="grid grid-cols-3 gap-2 sm:gap-3">
              {stats.map((s) => (
                <div
                  key={s.label}
                  className="rounded-xl border border-teyvat-line bg-teyvat-fill px-2 py-2 text-center"
                >
                  <p className="text-[17px] leading-none font-semibold text-teyvat-text tabular-nums">
                    {s.value}
                  </p>
                  <p className="mt-1.5 text-[11px] text-teyvat-text-4">{s.label}</p>
                </div>
              ))}
            </div>
          </div>
        )}
      </Card>

      {/* 角色选择条 + 选中角色详情（主从布局） */}
      {pool.length > 0 ? (
        <div className="space-y-5">
          <div>
            <h3 className="text-[13px] font-semibold tracking-[0.04em] text-teyvat-text-2">
              {t('profile.collectedCount', { count: pool.length })}
            </h3>
            {/* 两个口径都露出来：**已收录总数**（只增不减 ✓）+ **本轮展柜数**（≤12 ✓）
                ⇒ 用户才知道"换展柜还能继续导入、已收录的不会丢" ✓ */}
            <p className="mt-0.5 mb-2 text-[11px] leading-relaxed text-teyvat-text-4">
              {t('profile.showcaseHint', { showcase: profile.characters.length })}
            </p>
            {/* 只显示头像：一行能放下更多角色、纵向更省（名字/等级见下方角色概要与 title 提示）；
                已移出展柜的（inShowcase=false）**整体弱化**标出来 ✓ —— 它的数据停在 lastSeenAt 时刻，可能过时 ✗ */}
            <div className="-mx-1 flex gap-1.5 overflow-x-auto px-1 pb-1">
              {pool.map((c) => {
                const accent = elementColor(c.element)
                const isSelected = selected?.id === c.id
                const out = isCollected(c) && !c.inShowcase
                const tip = out
                  ? `${c.name} · Lv.${c.level} · ${t('profile.outOfShowcaseHint', { time: new Date(c.lastSeenAt).toLocaleDateString() })}`
                  : `${c.name} · Lv.${c.level}`
                return (
                  <button
                    key={c.id}
                    onClick={() => onSelect(c.id)}
                    title={tip}
                    aria-label={tip}
                    aria-pressed={isSelected}
                    className={clsx(
                      'relative shrink-0 rounded-xl border p-0.5 transition tx-ease tx-fast focus-visible:shadow-glow focus-visible:outline-none',
                      // 已移出展柜：弱化，一眼能看出"这张是历史数据"
                      out && 'opacity-60',
                      // 未选中用透明边框（而不是去掉边框），切换时不跳动
                      isSelected
                        ? 'border-teyvat-line-strong bg-teyvat-raised'
                        : 'border-transparent hover:border-teyvat-line-strong',
                    )}
                  >
                    {c.iconUrl ? (
                      <img
                        src={c.iconUrl}
                        alt={c.name}
                        width={48}
                        height={48}
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
                        className="absolute -right-0.5 -bottom-0.5 h-3.5 w-3.5 rounded-full border border-teyvat-ink"
                        style={{ background: accent }}
                        title={c.element}
                      />
                    )}
                    {/* 选中态：头像下方一条金线（不占额外高度） */}
                    {isSelected && (
                      <span className="absolute inset-x-1 -bottom-0.5 h-0.5 rounded-full bg-teyvat-gold" />
                    )}
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
                  <p className="text-[15px] font-semibold tracking-[0.01em] text-teyvat-text">
                    {selected.name}
                  </p>
                  <Chip dot={elementColor(selected.element)}>{selected.element}</Chip>
                </div>
                <p className="text-xs text-teyvat-text-2 tabular-nums">
                  Lv.{selected.level} ·{' '}
                  {t('character.constellation', { n: selected.constellation })}
                </p>
                <p className="text-xs text-teyvat-text-2 tabular-nums">
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
                {/* 已移出展柜的角色：面板会照旧渲染（历史数据也是数据 ✓），但**必须说清它停在什么时候** ✗
                    —— 否则用户会以为 Lv/圣遗物是实时的，据此做养成判断就会跑偏 ✓ */}
                {isCollected(selected) && !selected.inShowcase && (
                  <p className="w-full text-[11px] leading-relaxed text-teyvat-text-4">
                    {t('profile.outOfShowcaseHint', {
                      time: new Date(selected.lastSeenAt).toLocaleDateString(),
                    })}
                  </p>
                )}
              </div>

              {/* 面板属性 */}
              {selected.stats && selected.stats.length > 0 && (
                <div>
                  <h4 className="mb-2 text-[13px] font-semibold tracking-[0.04em] text-teyvat-text-2">
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
                              : 'border-teyvat-line bg-teyvat-fill',
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
                              'text-sm font-semibold tabular-nums',
                              highlighted ? 'text-teyvat-gold' : 'text-teyvat-text-2',
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
                  <h4 className="mb-1.5 text-[13px] font-semibold tracking-[0.04em] text-teyvat-text-2">
                    {t('character.relicsCount', { n: selected.relics.length })}
                  </h4>

                  {/* 一览：评分 + 套装构成 + 有效词条（参考提瓦特小助手，让"配置"第一眼就读得到） */}
                  <div className="mb-2 space-y-1.5">
                    <div className="flex flex-wrap items-center gap-1.5">
                      {/* 总评分（2026-09-27 起）：平均分 + 档位，悬停看口径说明；旧快照无字段不渲染 */}
                      {selected.relicScoreClass && selected.relicScoreAvg !== undefined && (
                        <Chip
                          tone="gold"
                          title={t('character.relicScoreNote')}
                          className="tabular-nums"
                        >
                          {t('character.relicScore')} {selected.relicScoreAvg.toFixed(1)} ·{' '}
                          {selected.relicScoreClass}
                        </Chip>
                      )}
                      {relicSets.map((g) => (
                        // 满 4 件用金色：4 件套是该套装的生效阈值
                        <Chip key={g.name} tone={g.count >= 4 ? 'gold' : undefined}>
                          {g.name} ×{g.count}
                        </Chip>
                      ))}
                    </div>
                    {selected.relicScoreTotal !== undefined && selected.relicScoreTotal > 0 && (
                      <div className="flex flex-wrap items-center gap-1.5">
                        <span className="text-[11px] text-teyvat-text-4">
                          {t('character.relicWords', {
                            n: (selected.relicWordCount ?? 0).toFixed(1),
                          })}
                          {' · '}
                          {t('character.relicCrit', {
                            n: (selected.relicCritScore ?? 0).toFixed(1),
                          })}
                        </span>
                      </div>
                    )}
                    {totalRolls > 0 && (
                      <div className="flex flex-wrap items-center gap-1.5">
                        <span className="text-[11px] text-teyvat-text-4">
                          {t('character.effectiveRolls', { n: totalRolls })}
                        </span>
                        {rollsByStat.slice(0, 4).map((x) => (
                          <span
                            key={x.name}
                            className="rounded-full border border-teyvat-line bg-teyvat-fill px-2 py-0.5 text-[11px] leading-tight text-teyvat-text-2 tabular-nums"
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
        <EmptyState
          icon={UserRound}
          title={t('account.emptyShowcase')}
          className="rounded-2xl border border-teyvat-line bg-teyvat-panel shadow-panel"
        />
      )}
    </div>
  )
}
