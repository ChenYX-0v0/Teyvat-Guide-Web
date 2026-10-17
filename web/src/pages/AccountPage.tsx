import { useCallback, useEffect, useRef, useState } from 'react'
import clsx from 'clsx'
import { Link2Off, Search, UserRound } from 'lucide-react'
import { genshinApi, type CollectedCharacter, type UIDBinding } from '@/api/genshin'
import PlayerProfileView from '@/components/account/PlayerProfileView'
import AnalysisFab from '@/components/account/AnalysisFab'
import Card from '@/components/common/Card'
import Button from '@/components/common/Button'
import Alert from '@/components/common/Alert'
import Skeleton from '@/components/common/Skeleton'
import { t } from '@/i18n'
import { EVENTS, track } from '@/lib/track'
import { resetAnalyzed } from '@/lib/analysisGate'
import type { PlayerProfile } from '@/types/genshin'

export default function AccountPage() {
  const [uid, setUid] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [profile, setProfile] = useState<PlayerProfile | null>(null)
  /**
   * 已收录角色池（2026-10-11）：**只增 / 只改，永不删** ✓。
   * 与 `profile.characters`（当前展柜 ✓）是两个口径 —— 用户轮换展柜就能把角色逐步全部导入 ✓。
   */
  const [collected, setCollected] = useState<CollectedCharacter[]>([])
  // 选中角色由本页持有：悬浮体检面板要知道"用户当前在看谁"（设计 §7.5）
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [binding, setBinding] = useState<UIDBinding | null>(null)
  // binding 的"上一帧"镜像：`doQuery` 是 useCallback([])（闭包会读到旧的 binding ✗），
  // 而"查询后是否**新产生**了绑定"必须拿旧值比 ⇒ 用 ref 才拿得到当前值 ✓（2026-10-12 口径 A ✓）
  const bindingRef = useRef<UIDBinding | null>(null)
  // 账号信息默认折叠：只留「标题 + 已绑定 UID」，解绑点开才出现（手机上把首屏留给展柜与圣遗物）
  const [infoOpen, setInfoOpen] = useState(false)

  // 2026-09-28：**去掉「强制刷新（绕过本地缓存）」入口** ✓ ——
  // 档案一律走服务端快照（TTL 10 分钟 / 资产 reload 时清），用户侧不需要绕过缓存的开关；
  // 该能力只留给运维排障：`GET /api/genshin/player?uid=…&refresh=true`（见 web/14 的资产核对配方）。
  const doQuery = useCallback(async (targetUid: string) => {
    if (!targetUid.trim()) return
    setLoading(true)
    setError('')
    setNotice('')
    setProfile(null)
    setCollected([])
    try {
      const p = await genshinApi.player(targetUid.trim())
      setProfile(p)
      // 埋点：只记结果，**不记 UID 原值**（合规 §8）
      // （原 force_refresh 字段随本功能一起去掉 ✓）
      track(EVENTS.accountUidQuery, {
        result: p.characters.length > 0 ? 'ok' : 'empty',
      })

      // 2026-10-12（口径 A）：**查询成功即自动绑定** ✓ —— 服务端在返回档案**之前**就把该 UID 绑到账号上了 ✓，
      // 这里把结果同步回界面（否则会出现"服务端已绑、界面还显示未绑定"的错位 ✗）。
      // ⚠️ 只在**绑定真的变了**时才提示 ✓ —— 反复查同一个已绑的 UID，不该每次都弹一次"绑定成功" ✗。
      try {
        const b = await genshinApi.binding()
        const prev = bindingRef.current?.uid ?? ''
        setBinding(b)
        bindingRef.current = b
        if (b.bound && b.uid && b.uid !== prev) {
          setNotice(t('account.bindSuccess', { uid: b.uid }))
          track(EVENTS.accountUidBind)
        }
      } catch {
        // 同步失败不影响档案展示 ✓（服务端已绑 ✓；下次进页面以 /binding 为准 ✓）
      }

      // 顺带拉"已收录角色池"（2026-10-11）：
      // ⚠️ **失败不打扰用户** ✓ —— 这只是增强（拿到就多显示历史角色）✓，
      // 拿不到就退回"只显示当前展柜" ✓，不能让主流程（档案展示）跟着失败 ✗。
      try {
        const col = await genshinApi.characters(p.uid)
        setCollected(col.characters)
      } catch {
        setCollected([])
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : t('errors.unknown'))
      track(EVENTS.accountUidQuery, { result: 'failed' })
    } finally {
      setLoading(false)
    }
  }, [])

  // 已绑定用户：进入页面自动加载档案，无需再输 UID
  useEffect(() => {
    genshinApi
      .binding()
      .then((b) => {
        setBinding(b)
        bindingRef.current = b
        if (b.bound && b.uid) void doQuery(b.uid)
      })
      .catch(() => {
        setBinding(null)
        bindingRef.current = null
      })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const query = useCallback(() => doQuery(uid), [doQuery, uid])

  // 2026-10-12（口径 A）：原先那个「绑定 {uid} 到我的账号」的**第二次点击已删掉** ✗ ——
  // 现在查询成功即由**服务端**自动绑定 ✓（见 doQuery），前端不再需要一个单独的绑定动作 ✓。
  // ⚠️ 后端 `POST /api/genshin/binding` **保留** ✓：用户浏览器里缓存的旧 bundle 还在发它 ✗，
  // 删接口只会让那些页面"点绑定报 404" ✓。

  const unbind = useCallback(async () => {
    setError('')
    setNotice('')
    try {
      await genshinApi.unbindUID()
      // 解绑 = 换了账号：体检记账随之清空（设计 §4.4）
      if (binding?.uid) resetAnalyzed(binding.uid)
      setBinding({ bound: false })
      bindingRef.current = { bound: false }
      setProfile(null)
      // 角色池是**按游戏 UID** 存的 ✓：解绑后本地不再显示（服务端刻意不删 ✓，换回同一 UID 还在 ✓）
      setCollected([])
      setUid('')
      setNotice(t('account.unbindSuccess'))
      track(EVENTS.accountUnbind)
    } catch (e) {
      setError(e instanceof Error ? e.message : t('errors.unknown'))
    }
  }, [binding])

  // 角色条与体检面板都用这个集合：优先已收录池 ✓，拿不到才退回当前展柜 ✓
  const pool: CollectedCharacter[] | NonNullable<PlayerProfile['characters']> =
    collected.length > 0 ? collected : (profile?.characters ?? [])

  return (
    // 底部留出悬浮体检按钮的位置，避免它压住最后一排圣遗物
    <div className="mx-auto max-w-5xl space-y-6 px-4 pt-6 pb-24 sm:px-6 sm:pt-8 sm:pb-8">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2.5">
          <span className="flex h-9 w-9 items-center justify-center rounded-xl border border-teyvat-line-strong bg-teyvat-gold/10 text-teyvat-gold">
            <UserRound className="h-4 w-4" />
          </span>
          <h2 className="text-[22px] leading-tight font-semibold tracking-[0.01em] text-teyvat-gold">
            {t('account.title')}
          </h2>
        </div>

        {/* 已绑定：一行摘要（UID + 展开按钮）；解绑在展开后出现，隐藏查询卡 */}
        {binding?.bound && (
          <div className="flex items-center gap-2 rounded-xl border border-teyvat-line-strong bg-teyvat-gold/5 px-3 py-1.5">
            <span className="text-xs text-teyvat-text-3">{t('account.boundLabel')}</span>
            <span className="text-sm font-semibold text-teyvat-gold tabular-nums">
              {binding.uid}
            </span>
            {infoOpen && (
              <>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={unbind}
                  className="hover:border-err/40 hover:text-err"
                >
                  <Link2Off className="h-3 w-3" />
                  {t('account.unbind')}
                </Button>
              </>
            )}
            <button
              type="button"
              onClick={() => setInfoOpen((v) => !v)}
              aria-expanded={infoOpen}
              aria-label={infoOpen ? t('common.collapse') : t('common.expand')}
              className="tap-44 rounded-lg p-1 text-teyvat-text-4 transition tx-ease tx-fast hover:text-teyvat-gold focus-visible:shadow-glow focus-visible:outline-none"
            >
              <span className={clsx('block text-[10px] leading-none', infoOpen && 'rotate-180')}>▾</span>
            </button>
          </div>
        )}
      </div>

      {/* 未绑定：显示查询卡（**查询成功即自动绑定** ✓，2026-10-12 口径 A） */}
      {!binding?.bound && (
        <Card className="tx-enter p-5">
          <h3 className="text-[13px] font-semibold tracking-[0.04em] text-teyvat-text-2">
            {t('account.queryTitle')}
          </h3>
          <div className="mt-3 flex gap-2">
            <input
              value={uid}
              onChange={(e) => setUid(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && query()}
              placeholder={t('account.queryPlaceholder')}
              className="min-w-0 flex-1 rounded-xl border border-teyvat-line bg-teyvat-fill px-4 py-2.5 text-sm text-teyvat-text outline-none transition tx-ease tx-fast placeholder:text-teyvat-text-4 hover:border-teyvat-line-strong focus:border-teyvat-line-strong focus:shadow-glow"
            />
            <Button variant="primary" onClick={query} disabled={loading || !uid.trim()}>
              <Search className="h-4 w-4" />
              {loading ? t('common.querying') : t('common.query')}
            </Button>
          </div>
          {/* 2026-10-12（口径 A）：这里原先有个"绑定 {uid} 到我的账号"的第二次点击 —— 已删掉 ✗，
              查询成功即自动绑定 ✓。改成**事先告知**的一行小字：让用户知道"查询"会顺手绑定 ✓
              （不搞暗箱操作 ✗；反悔路径也在文案里写明：展开「已绑定 UID」可解绑 ✓）。 */}
          <p className="mt-2 text-[11px] leading-relaxed text-teyvat-text-4">
            {t('account.queryAutoBindHint')}
          </p>
        </Card>
      )}

      {/* 查询中：骨架直接画出「档案」的形状（比一行"查询中…"少一次布局跳动） */}
      {loading && !profile && <ProfileSkeleton />}

      {notice && <Alert tone="ok">{notice}</Alert>}

      {error && (
        <Alert
          tone="error"
          action={
            <button
              type="button"
              onClick={query}
              disabled={loading}
              className="rounded-lg border border-err/40 px-2 py-1 text-[11px] transition tx-ease tx-fast hover:bg-err/10 focus-visible:shadow-glow focus-visible:outline-none disabled:opacity-40"
            >
              {t('analysis.retry')}
            </button>
          }
        >
          {error}
        </Alert>
      )}

      {profile && (
        <PlayerProfileView
          profile={profile}
          collected={collected}
          selectedId={selectedId}
          onSelect={setSelectedId}
        />
      )}

      {/* 悬浮体检入口：未绑定 UID 时置灰而不是隐藏（用户需要看到"为什么不能问"）；
          可选角色用**已收录池** ⇒ 换出展柜的老角色也能做体检 ✓ */}
      <AnalysisFab
        uid={binding?.uid ?? ''}
        characters={pool}
        character={pool.find((c) => c.id === selectedId) ?? pool[0]}
      />
    </div>
  )
}

/**
 * 查询中的骨架（2026-10-17 · UI 优化第 1 轮）：
 * 直接画出「玩家头部 + 角色条 + 属性格」的形状 —— 比一行「查询中…」少一次布局跳动。
 */
function ProfileSkeleton() {
  return (
    <div className="space-y-5" aria-hidden>
      <div className="rounded-2xl border border-teyvat-line bg-teyvat-panel p-3 shadow-panel sm:p-4">
        <div className="flex items-center gap-3">
          <Skeleton className="h-10 w-10 rounded-xl" />
          <div className="space-y-1.5">
            <Skeleton className="h-3.5 w-32" />
            <Skeleton className="h-2.5 w-20" />
          </div>
          <Skeleton className="ml-auto h-6 w-20 rounded-full" />
        </div>
      </div>
      <div className="space-y-3">
        <Skeleton className="h-3 w-28" />
        <div className="flex gap-2">
          {[0, 1, 2, 3, 4, 5].map((i) => (
            <Skeleton key={i} className="h-12 w-12 rounded-lg" />
          ))}
        </div>
      </div>
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
        {[0, 1, 2, 3, 4, 5, 6, 7].map((i) => (
          <Skeleton key={i} className="h-9 rounded-xl" />
        ))}
      </div>
    </div>
  )
}
