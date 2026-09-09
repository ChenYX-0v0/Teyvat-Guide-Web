import { AlertTriangle } from 'lucide-react'
import CharacterCard from './CharacterCard'
import { t } from '@/i18n'
import type { PlayerProfile } from '@/types/genshin'

export default function PlayerProfileView({ profile }: { profile: PlayerProfile }) {
  const stats = [
    { label: t('profile.statsAchievements'), value: profile.achievements },
    { label: t('profile.statsWorldLevel'), value: profile.worldLevel },
    { label: t('profile.statsAvatars'), value: profile.characters.length },
  ]

  return (
    <div className="space-y-6">
      {profile.warnings && profile.warnings.length > 0 && (
        <div className="rounded-2xl border border-amber-400/30 bg-amber-400/10 p-4 text-xs text-amber-200">
          <p className="flex items-center gap-1.5 font-medium">
            <AlertTriangle className="h-3.5 w-3.5" />
            {t('profile.partialFailed')}
          </p>
          <ul className="mt-1.5 list-inside list-disc space-y-0.5 text-amber-200/80">
            {profile.warnings.map((w) => (
              <li key={w}>{w}</li>
            ))}
          </ul>
        </div>
      )}

      {/* 玩家头部 */}
      <div className="flex flex-wrap items-center gap-5 rounded-2xl border border-white/10 bg-teyvat-panel/60 p-5">
        {profile.avatarUrl ? (
          <img
            src={profile.avatarUrl}
            alt={profile.nickname}
            className="h-16 w-16 rounded-2xl border border-teyvat-gold/30 object-cover"
          />
        ) : (
          <div className="flex h-16 w-16 items-center justify-center rounded-2xl border border-teyvat-gold/30 bg-teyvat-gold/10 text-xl font-semibold text-teyvat-gold">
            {profile.nickname[0] ?? '?'}
          </div>
        )}
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <h2 className="text-lg font-semibold text-white">{profile.nickname}</h2>
            <span className="rounded-md bg-teyvat-gold/15 px-2 py-0.5 text-xs text-teyvat-gold">
              Lv.{profile.level}
            </span>
          </div>
          <p className="mt-1 text-xs text-white/45">UID {profile.uid}</p>
          <p className="mt-1 max-w-md truncate text-xs text-white/40">
            {profile.signature || t('profile.noSignature')}
          </p>
        </div>
        <div className="ml-auto grid grid-cols-3 gap-3">
          {stats.map((s) => (
            <div key={s.label} className="rounded-xl bg-white/5 px-4 py-2 text-center">
              <p className="text-base font-semibold text-white">{s.value}</p>
              <p className="mt-0.5 text-xs text-white/45">{s.label}</p>
            </div>
          ))}
        </div>
      </div>

      {/* 角色列表 */}
      {profile.characters.length > 0 ? (
        <div>
          <h3 className="mb-3 text-sm font-medium text-white">
            {t('profile.charactersCount', { count: profile.characters.length })}
          </h3>
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
            {profile.characters.map((c) => (
              <CharacterCard key={c.id} c={c} />
            ))}
          </div>
        </div>
      ) : (
        <div className="rounded-2xl border border-white/10 bg-teyvat-panel/60 p-8 text-center text-sm text-white/40">
          {t('account.emptyShowcase')}
        </div>
      )}
    </div>
  )
}
