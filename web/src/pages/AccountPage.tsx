import { useCallback, useEffect, useState } from 'react'
import clsx from 'clsx'
import { Link2Off, RefreshCw, Search, UserRound } from 'lucide-react'
import { genshinApi, type UIDBinding } from '@/api/genshin'
import PlayerProfileView from '@/components/account/PlayerProfileView'
import Card from '@/components/common/Card'
import Button from '@/components/common/Button'
import { t } from '@/i18n'
import { EVENTS, track } from '@/lib/track'
import type { PlayerProfile } from '@/types/genshin'

export default function AccountPage() {
  const [uid, setUid] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [profile, setProfile] = useState<PlayerProfile | null>(null)
  const [refresh, setRefresh] = useState(false)
  const [binding, setBinding] = useState<UIDBinding | null>(null)
  // 账号信息默认折叠：只留「标题 + 已绑定 UID」，刷新/解绑点开才出现（手机上把首屏留给展柜与圣遗物）
  const [infoOpen, setInfoOpen] = useState(false)

  const doQuery = useCallback(async (targetUid: string, forceRefresh = false) => {
    if (!targetUid.trim()) return
    setLoading(true)
    setError('')
    setNotice('')
    setProfile(null)
    try {
      const p = await genshinApi.player(targetUid.trim(), forceRefresh)
      setProfile(p)
      // 埋点：只记结果与是否强制刷新，**不记 UID 原值**（合规 §8）
      track(EVENTS.accountUidQuery, {
        result: p.characters.length > 0 ? 'ok' : 'empty',
        force_refresh: forceRefresh,
      })
    } catch (e) {
      setError(e instanceof Error ? e.message : t('errors.unknown'))
      track(EVENTS.accountUidQuery, { result: 'failed', force_refresh: forceRefresh })
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
        if (b.bound && b.uid) void doQuery(b.uid)
      })
      .catch(() => setBinding(null))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const query = useCallback(() => doQuery(uid, refresh), [doQuery, uid, refresh])

  const bind = useCallback(async () => {
    setError('')
    setNotice('')
    try {
      const b = await genshinApi.bindUID(uid.trim())
      setBinding(b)
      setNotice(t('account.bindSuccess', { uid: b.uid ?? uid.trim() }))
      track(EVENTS.accountUidBind)
    } catch (e) {
      setError(e instanceof Error ? e.message : t('errors.unknown'))
    }
  }, [uid])

  const unbind = useCallback(async () => {
    setError('')
    setNotice('')
    try {
      await genshinApi.unbindUID()
      setBinding({ bound: false })
      setProfile(null)
      setUid('')
      setNotice(t('account.unbindSuccess'))
      track(EVENTS.accountUnbind)
    } catch (e) {
      setError(e instanceof Error ? e.message : t('errors.unknown'))
    }
  }, [])

  return (
    <div className="mx-auto max-w-5xl space-y-6 px-6 py-8">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="flex items-center gap-2 text-xl font-semibold text-teyvat-gold">
          <UserRound className="h-5 w-5" />
          {t('account.title')}
        </h2>

        {/* 已绑定：一行摘要（UID + 展开按钮）；刷新 / 解绑在展开后出现，隐藏查询卡 */}
        {binding?.bound && (
          <div className="flex items-center gap-2 rounded-xl border border-teyvat-line-strong bg-teyvat-gold/5 px-3 py-1.5">
            <span className="text-xs text-teyvat-text-3">{t('account.boundLabel')}</span>
            <span className="text-sm font-semibold text-teyvat-gold">{binding.uid}</span>
            {infoOpen && (
              <>
                <button
                  onClick={() => binding.uid && doQuery(binding.uid, true)}
                  disabled={loading}
                  title={t('account.forceRefresh')}
                  className="rounded-lg p-1 text-teyvat-text-3 transition duration-150 hover:text-teyvat-gold focus-visible:shadow-glow focus-visible:outline-none disabled:opacity-40"
                >
                  <RefreshCw className={loading ? 'h-3.5 w-3.5 animate-spin' : 'h-3.5 w-3.5'} />
                </button>
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
              className="rounded-lg p-1 text-teyvat-text-4 transition duration-150 hover:text-teyvat-gold focus-visible:shadow-glow focus-visible:outline-none"
            >
              <span className={clsx('block text-[10px] leading-none', infoOpen && 'rotate-180')}>▾</span>
            </button>
          </div>
        )}
      </div>

      {/* 未绑定：显示查询卡（查询成功后可绑定） */}
      {!binding?.bound && (
        <Card className="p-5">
          <h3 className="text-sm font-medium text-teyvat-text">{t('account.queryTitle')}</h3>
          <div className="mt-3 flex gap-2">
            <input
              value={uid}
              onChange={(e) => setUid(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && query()}
              placeholder={t('account.queryPlaceholder')}
              className="flex-1 rounded-xl border border-teyvat-line bg-white/[0.04] px-4 py-2.5 text-sm text-teyvat-text outline-none transition placeholder:text-teyvat-text-4 focus:border-teyvat-line-strong focus:shadow-glow"
            />
            <Button variant="primary" onClick={query} disabled={loading || !uid.trim()}>
              <Search className="h-4 w-4" />
              {loading ? t('common.querying') : t('common.query')}
            </Button>
          </div>
          <label className="mt-2 flex cursor-pointer items-center gap-1.5 text-xs text-teyvat-text-3 select-none">
            <input
              type="checkbox"
              checked={refresh}
              onChange={(e) => setRefresh(e.target.checked)}
              className="h-3.5 w-3.5 accent-teyvat-gold"
            />
            {t('account.forceRefresh')}
          </label>
          {uid.trim() && profile && (
            <button
              onClick={bind}
              className="mt-2 text-xs text-teyvat-gold transition duration-150 hover:text-teyvat-gold-hi focus-visible:shadow-glow focus-visible:outline-none"
            >
              {t('account.bind', { uid: uid.trim() })}
            </button>
          )}
        </Card>
      )}

      {notice && (
        <div className="rounded-2xl border border-teyvat-line-strong bg-teyvat-gold/5 p-4 text-sm text-teyvat-gold-hi">
          {notice}
        </div>
      )}

      {error && (
        <div className="rounded-2xl border border-err/30 bg-err/10 p-4 text-sm text-err">{error}</div>
      )}

      {profile && <PlayerProfileView profile={profile} />}
    </div>
  )
}
