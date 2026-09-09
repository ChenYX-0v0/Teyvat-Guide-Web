import { useCallback, useState } from 'react'
import { Search, UserRound } from 'lucide-react'
import { genshinApi } from '@/api/genshin'
import PlayerProfileView from '@/components/account/PlayerProfileView'
import { t } from '@/i18n'
import type { PlayerProfile } from '@/types/genshin'

export default function AccountPage() {
  const [uid, setUid] = useState('197794747')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [profile, setProfile] = useState<PlayerProfile | null>(null)
  const [refresh, setRefresh] = useState(false)

  const query = useCallback(
    async (targetUid = uid, forceRefresh = refresh) => {
      if (!targetUid.trim()) return
      setLoading(true)
      setError('')
      setProfile(null)
      try {
        setProfile(await genshinApi.player(targetUid.trim(), forceRefresh))
      } catch (e) {
        setError(e instanceof Error ? e.message : t('errors.unknown'))
      } finally {
        setLoading(false)
      }
    },
    [uid, refresh],
  )

  return (
    <div className="mx-auto max-w-5xl space-y-6 px-6 py-8">
      <div>
        <h2 className="flex items-center gap-2 text-xl font-semibold text-teyvat-gold">
          <UserRound className="h-5 w-5" />
          {t('account.title')}
        </h2>
        <p className="mt-1 text-sm text-white/50">{t('account.intro')}</p>
      </div>

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
            onClick={() => query()}
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
      </section>

      {error && (
        <div className="rounded-2xl border border-red-400/30 bg-red-400/10 p-4 text-sm text-red-300">
          {error}
        </div>
      )}

      {profile && <PlayerProfileView profile={profile} />}
    </div>
  )
}
