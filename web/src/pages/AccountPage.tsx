import { useCallback, useEffect, useRef, useState } from 'react'
import { Link2Off, RefreshCw, Search, UserRound } from 'lucide-react'
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
  /** 本地快照的落库时间（服务端随 `/characters` 带回 ✓；空串 = 还没同步过） */
  const [syncedAt, setSyncedAt] = useState('')
  /** 「已绑定但库里一条数据都没有」时，引导用户做那一次手动同步（见 loadFromDb） */
  const [loadedFromDb, setLoadedFromDb] = useState(false)
  /**
   * `binding()` 是否**已经有结论**（2026-10-18）。
   *
   * ⚠️ 不能用 `binding === null` 代替：初值就是 `null`，而绑定查询是异步的 ⇒
   * 挂载后到响应回来之间，`!binding?.bound` 也成立，于是**未绑定用户才该看到的 UID 输入卡
   * 会先闪一下**，等绑定结果回来才消失 ✗。必须等"确实没绑定"再渲染那张卡 ✓。
   */
  const [bindingReady, setBindingReady] = useState(false)
  /** 兜底卡片里的解绑二次确认（正常路径的解绑在玩家头部卡里，见 PlayerProfileView） */
  const [confirmUnbindFallback, setConfirmUnbindFallback] = useState(false)
  // 选中角色由本页持有：悬浮体检面板要知道"用户当前在看谁"（设计 §7.5）
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [binding, setBinding] = useState<UIDBinding | null>(null)
  // binding 的"上一帧"镜像：`doQuery` 是 useCallback([])（闭包会读到旧的 binding ✗），
  // 而"查询后是否**新产生**了绑定"必须拿旧值比 ⇒ 用 ref 才拿得到当前值 ✓（2026-10-12 口径 A ✓）
  const bindingRef = useRef<UIDBinding | null>(null)
  // 2026-10-18：「已绑定 UID」那枚胶囊卡已**整体移除** ✓ —— UID 本来就印在玩家头部卡里
  // （`PlayerProfileView` 折叠行右侧「UID …」），再放一个只是重复。
  // 「同步展柜 / 解除绑定」两个动作随之挪进**玩家头部卡展开区**（折叠态不占地方 ✓）。
  // 解绑的二次确认也一起搬过去，由 `PlayerProfileView` 自己持有 ✓。

  /**
   * 手动同步（2026-10-18 口径）：**这是全站唯一会打 Enka 上游的入口** ✓。
   *
   * 触发点只有两个：① 已绑定用户点「同步展柜」；② 用户输入 UID 点「查询」。
   * 页面打开时**不**走这里（走 `loadFromDb` 只读库 ✓）—— 此前进页面就自动打一次上游，
   * 还会因为"先渲染展柜、再被累积池覆盖"导致列表看起来重排了一次 ✗。
   *
   * `refresh=true`：手动同步必须跳过快照缓存，否则用户点了"同步"却拿到 10 分钟内的旧数据 ✗。
   */
  const doQuery = useCallback(async (targetUid: string, opts: { refresh?: boolean } = {}) => {
    if (!targetUid.trim()) return
    setLoading(true)
    setError('')
    setNotice('')
    setProfile(null)
    setCollected([])
    try {
      const p = await genshinApi.player(targetUid.trim(), opts)
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

      // 同步完之后拉一次"已收录角色池"（采集已在服务端 `/player` 里做完 ✓）：
      // ⚠️ **失败不打扰用户** ✓ —— 这只是增强（拿到就多显示历史角色）✓，
      // 拿不到就退回"只显示当前展柜" ✓，不能让主流程（档案展示）跟着失败 ✗。
      try {
        const col = await genshinApi.characters(p.uid)
        setCollected(col.characters)
        setSyncedAt(col.syncedAt ?? '')
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

  /**
   * 账号页的**唯一数据入口**（2026-10-18 口径）：进页面 / 已绑定用户默认走这条。
   *
   * 此前进页面直接 `doQuery()` ⇒ 打 `/player`（可能打到上游 Enka）+ 再打 `/characters`，
   * 两个响应先后到达 ⇒ 列表先按展柜顺序出现、几百毫秒后又被累积池覆盖成另一个顺序，
   * 看起来像"自己又排了一次" ✗。现在只有这一个请求 ✓，一次绘制 ✓。
   *
   * 服务端对这个接口是 cache-aside：库里有就返回、库空自动回源 Enka 并入库 ✓
   * ⇒ 前端不需要判断"要不要提示用户先同步" ✗（那是把系统的事推给用户 ✗）。
   * 昵称 / 等级 / 头像来自响应里的 `profile`（服务端从落库快照带出 ✓）。
   */
  const loadFromDb = useCallback(async (targetUid: string, opts: { refresh?: boolean } = {}) => {
    if (!targetUid) return
    setLoading(true)
    try {
      // 服务端是 cache-aside：库里有就返回、库空会自动回源 Enka 并入库 ✓
      // ⇒ 前端只管调这一个接口，**不需要**再判断"有没有、要不要提示用户同步" ✗
      const col = await genshinApi.characters(targetUid, opts)
      setCollected(col.characters)
      if (col.profile) setProfile(col.profile)
      setSyncedAt(col.syncedAt ?? '')
    } catch {
      // 拉取失败（上游超时 / 无公开展柜）不弹错：页面给"重试"入口，别整页报错 ✗
      setCollected([])
    } finally {
      setLoading(false)
      setLoadedFromDb(true)
    }
  }, [])

  // 已绑定用户：进入页面**只读库**，不再自动打 Enka ✓（上游只由手动同步触发）
  useEffect(() => {
    genshinApi
      .binding()
      .then((b) => {
        setBinding(b)
        bindingRef.current = b
        setBindingReady(true)
        if (b.bound && b.uid) void loadFromDb(b.uid)
        else setLoadedFromDb(true)
      })
      .catch(() => {
        setBinding(null)
        bindingRef.current = null
        setBindingReady(true)
        setLoadedFromDb(true)
      })
  }, [loadFromDb])

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
          <div>
            <h2 className="text-[22px] leading-tight font-semibold tracking-[0.01em] text-teyvat-gold">
              {t('account.title')}
            </h2>
            {/* 只读库口径下必须让用户知道"这是上次同步的结果"（2026-10-18） */}
            {syncedAt ? (
              <p className="mt-0.5 text-[11px] text-teyvat-text-4">
                {t('account.syncedAt', { time: new Date(syncedAt).toLocaleString() })}
              </p>
            ) : null}
          </div>
        </div>

      </div>

      {/* 回源没成功（上游超时 / 该 UID 没有公开展柜 / 从没同步过）⇒ 给一个**重试**入口 ✓
          ⚠️ 这里是"自动回源失败"的兜底，不是"请用户手动同步"的前置要求 —— 正常路径由服务端
          cache-aside 自动完成 ✓。判据是「没有 profile」：档案头部来自快照，没它就渲染不出角色区 ✗ */}
      {binding?.bound && loadedFromDb && !loading && !profile && (
        <Card className="p-5">
          <h3 className="text-sm font-medium text-teyvat-text">{t('account.syncEmptyTitle')}</h3>
          <div className="mt-3 flex flex-wrap items-center gap-2">
            <Button
              variant="primary"
              disabled={loading}
              onClick={() => void loadFromDb(binding.uid ?? '', { refresh: true })}
            >
              <RefreshCw className="h-4 w-4" />
              {loading ? t('common.querying') : t('account.resync')}
            </Button>
            {/* 兜底出口：没有档案时玩家头部卡渲染不出来，解绑不能因此变成死路 ✓ */}
            {confirmUnbindFallback ? (
              <>
                <span className="text-[11px] text-err">{t('account.unbindConfirm')}</span>
                <Button
                  variant="ghost"
                  size="sm"
                  className="border-err/50 text-err hover:border-err hover:bg-err/10 hover:text-err"
                  onClick={() => {
                    setConfirmUnbindFallback(false)
                    void unbind()
                  }}
                >
                  {t('account.unbindConfirmYes')}
                </Button>
                <Button variant="ghost" size="sm" onClick={() => setConfirmUnbindFallback(false)}>
                  {t('common.cancel')}
                </Button>
              </>
            ) : (
              <Button
                variant="ghost"
                size="sm"
                className="hover:border-err/40 hover:text-err"
                onClick={() => setConfirmUnbindFallback(true)}
              >
                <Link2Off className="h-3 w-3" />
                {t('account.unbind')}
              </Button>
            )}
          </div>
        </Card>
      )}

      {/* 未绑定：显示查询卡（**查询成功即自动绑定** ✓，2026-10-12 口径 A）
          ⚠️ 必须等 bindingReady：否则绑定查询返回前会先闪一下这张卡（已绑定用户也能看到输入框 ✗） */}
      {bindingReady && !binding?.bound && (
        <Card className="tx-enter p-5">
          <h3 className="text-[13px] font-semibold tracking-[0.04em] text-teyvat-text-2">
            {t('account.queryTitle')}
          </h3>
          <div className="mt-3 flex gap-2">
            <input
              name="uid"
              aria-label={t('account.uidInputLabel')}
              inputMode="numeric"
              autoComplete="off"
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

      {/* 查询中：骨架直接画出「档案」的形状（比一行"查询中…"少一次布局跳动）
          绑定态未就绪时也走骨架：这一小段既不该出现未绑定查询卡，也不该是一片空白 ✓ */}
      {(!bindingReady || (loading && !profile)) && <ProfileSkeleton />}

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
          // 同步 / 解绑原先在页面右上那枚「已绑定 UID」胶囊里，2026-10-18 挪进玩家头部卡展开区
          onSync={() => void loadFromDb(binding?.uid ?? profile.uid, { refresh: true })}
          syncing={loading}
          onUnbind={() => void unbind()}
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
