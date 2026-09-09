import { useCallback, useEffect, useState } from 'react'
import { Link2Off, RefreshCw, Search, UserRound } from 'lucide-react'
import { genshinApi, type UIDBinding } from '@/api/genshin'
import PlayerProfileView from '@/components/account/PlayerProfileView'
import { t } from '@/i18n'
import type { PlayerProfile } from '@/types/genshin'

export default function AccountPage() {
  const [uid, setUid] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [profile, setProfile] = useState<PlayerProfile | null>(null)
  const [refresh, setRefresh] = useState(false)
  const [binding, setBinding] = useState<UIDBinding | null>(null)

  const doQuery = useCallback(async (targetUid: string, forceRefresh = false) => {
    if (!targetUid.trim()) return
    setLoading(true)
    setError('')
    setNotice('')
    setProfile(null)
    try {
      setProfile(await genshinApi.player(targetUid.trim(), forceRefresh))
    } catch (e) {
      setError(e instanceof Error ? e.message : t('errors.unknown'))
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
    } catch (e) {
      setError(e instanceof Error ? e.message : t('errors.unknown'))
    }
  }, [])

  return (
    <div className="mx-auto max-w-5xl space-y-6 px-6 py-8">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h2 className="flex items-center gap-2 text-xl font-semibold text-teyvat-gold">
            <UserRound className="h-5 w-5" />
            {t('account.title')}
          </h2>
        </div>

        {/* 已绑定：左上角小卡片（UID + 刷新 + 解绑），隐藏查询卡 */}
        {binding?.bound && (
          <div className="flex items-center gap-2 self-start rounded-xl border border-teyvat-gold/25 bg-teyvat-gold/5 px-3 py-2">
            <span className="text-xs text-white/55">{t('account.boundLabel')}</span>
            <span className="text-sm font-semibold text-teyvat-gold">{binding.uid}</span>
            <button
              onClick={() => binding.uid && doQuery(binding.uid, true)}
              disabled={loading}
              title={t('account.forceRefresh')}
              className="rounded-lg p-1 text-white/50 transition hover:text-teyvat-gold disabled:opacity-40"
            >
              <RefreshCw className={loading ? 'h-3.5 w-3.5 animate-spin' : 'h-3.5 w-3.5'} />
            </button>
            <button
              onClick={unbind}
              className="flex items-center gap-1 rounded-lg border border-white/15 px-2 py-1 text-xs text-white/50 transition hover:border-red-400/40 hover:text-red-300"
            >
              <Link2Off className="h-3 w-3" />
              {t('account.unbind')}
            </button>
          </div>
        )}
      </div>

      {/* 未绑定：显示查询卡（查询成功后可绑定） */}
      {!binding?.bound && (
        <section className="rounded-2xl border border-white/10 bg-teyvat-panel/60 p-5">
          <h3 className="text-sm font-medium text-white">{t('account.queryTitle')}</h3>
          <div className="mt-3 flex gap-2">
            <input
              value={uid}
              onChange={(e) => setUid(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && query()}
              placeholder={t('account.queryPlaceholder')}
              className="flex-1 rounded-xl border border-white/10 bg-white/5 px-4 py-2.5 text-sm outline-none transition placeholder:text-white/25 focus:border-teyvat-gold/50"
            />
            <button
              onClick={query}
              disabled={loading || !uid.trim()}
              className="flex items-center gap-1.5 rounded-xl bg-teyvat-gold px-5 py-2.5 text-sm font-medium text-teyvat-ink transition hover:bg-teyvat-gold-deep disabled:cursor-not-allowed disabled:opacity-40"
            >
              <Search className="h-4 w-4" />
              {loading ? t('common.querying') : t('common.query')}
            </button>
          </div>
          <label className="mt-2 flex cursor-pointer items-center gap-1.5 text-xs text-white/45 select-none">
            <input
              type="checkbox"
              checked={refresh}
              onChange={(e) => setRefresh(e.target.checked)}
              className="h-3.5 w-3.5 accent-[#d3bc8e]"
            />
            {t('account.forceRefresh')}
          </label>
          {uid.trim() && profile && (
            <button
              onClick={bind}
              className="mt-2 text-xs text-teyvat-gold transition hover:text-teyvat-gold-deep"
            >
              {t('account.bind', { uid: uid.trim() })}
            </button>
          )}
        </section>
      )}

      {notice && (
        <div className="rounded-2xl border border-teyvat-gold/25 bg-teyvat-gold/5 p-4 text-sm text-teyvat-gold">
          {notice}
        </div>
      )}

      {error && (
        <div className="rounded-2xl border border-red-400/30 bg-red-400/10 p-4 text-sm text-red-300">
          {error}
        </div>
      )}

      {profile && <PlayerProfileView profile={profile} />}
    </div>
  )
}
