import { useEffect, useState, type FormEvent } from 'react'
import Modal from '@/components/common/Modal'
import Button from '@/components/common/Button'
import { authApi, type MeResult } from '@/api/auth'
import { useAuthStore } from '@/store/useAuthStore'
import { t } from '@/i18n'

type Props = {
  open: boolean
  onClose: () => void
  /** 跳页面（账号信息 / 用户须知）：由 UserCard 负责"先关弹窗与抽屉、再跳" ✓ */
  onNavigate: (path: string) => void
}

/**
 * 个人信息管理（侧栏用户卡片的第一个功能，2026-10-11）。
 *
 * 能改的只有**昵称** ✓ —— 用户名（手机号）是登录凭据，不可改 ✗；
 * 游戏 UID 绑定是账号页的活儿，这里只显示 + 给个入口 ✓（不在两个地方各做一套 ✗）。
 *
 * ⚠️ 用户名与绑定 UID **不在 auth store 里**，所以打开时拉一次 `/auth/me` ✓：
 * 这次请求很轻，为它去扩全局 state 不划算 ✓（而且拉到的总是最新值 ✓）。
 */
export default function ProfileModal({ open, onClose, onNavigate }: Props) {
  const updateDisplayName = useAuthStore((s) => s.updateDisplayName)

  const [me, setMe] = useState<MeResult | null>(null)
  const [name, setName] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [saved, setSaved] = useState(false)
  const [loaded, setLoaded] = useState(false)

  useEffect(() => {
    if (!open) return
    setError('')
    setSaved(false)
    setMe(null)
    setLoaded(false)
    authApi
      .me()
      .then((r) => {
        setMe(r)
        setName(r.displayName ?? '')
        setLoaded(true)
      })
      .catch(() => setError(t('profile.loadFailed')))
  }, [open])

  const save = async (e: FormEvent) => {
    e.preventDefault()
    setBusy(true)
    setError('')
    setSaved(false)
    try {
      // 以服务端返回值为准（服务端会 TrimSpace ✓，可能与我传进去的不完全一样 ✓）
      setName(await updateDisplayName(name))
      setSaved(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : t('errors.unknown'))
    } finally {
      setBusy(false)
    }
  }

  // 输入框字号移动端 ≥16px：iOS 聚焦更小字号会自动放大页面（与登录页同一口径 ✓）
  const inputClass =
    'mt-1 w-full rounded-xl border border-teyvat-line bg-white/[0.04] px-3 py-2 text-base text-teyvat-text outline-none transition placeholder:text-teyvat-text-4 focus:border-teyvat-line-strong focus:shadow-glow sm:text-sm'

  return (
    <Modal
      open={open}
      title={t('profile.title')}
      onClose={onClose}
      footer={
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="flex items-center gap-2">
            <Button variant="ghost" size="sm" onClick={() => onNavigate('/account')}>
              {t('profile.goAccount')}
            </Button>
            <Button variant="ghost" size="sm" onClick={() => onNavigate('/terms')}>
              {t('profile.terms')}
            </Button>
          </div>
          <Button variant="primary" type="submit" form="profile-form" disabled={busy || !loaded}>
            {t('common.save')}
          </Button>
        </div>
      }
    >
      <form id="profile-form" onSubmit={save} className="space-y-4">
        <div>
          <span className="text-xs text-teyvat-text-3">{t('profile.usernameLabel')}</span>
          <p className="mt-1 truncate rounded-xl border border-teyvat-line bg-white/[0.02] px-3 py-2 text-sm text-teyvat-text-2">
            {me?.username ?? '—'}
          </p>
        </div>

        <div>
          <label htmlFor="profile-name" className="text-xs text-teyvat-text-3">
            {t('profile.nickname')}
          </label>
          <input
            id="profile-name"
            value={name}
            onChange={(e) => {
              setName(e.target.value)
              setSaved(false)
            }}
            // 服务端上限是 24 个**字符**；这里放宽到 40 只是防手滑，真正说了算的是服务端 ✓
            maxLength={40}
            placeholder={me?.username ?? ''}
            className={inputClass}
          />
          <p className="mt-1 text-[11px] leading-relaxed text-teyvat-text-4">{t('profile.nicknameHint')}</p>
        </div>

        <div>
          <span className="text-xs text-teyvat-text-3">{t('profile.uidLabel')}</span>
          <p className="mt-1 truncate rounded-xl border border-teyvat-line bg-white/[0.02] px-3 py-2 text-sm text-teyvat-text-2">
            {me?.genshinUid ? me.genshinUid : t('profile.notBound')}
          </p>
        </div>

        {error ? <p className="text-xs text-err">{error}</p> : null}
        {saved ? <p className="text-xs text-ok">{t('profile.saved')}</p> : null}
      </form>
    </Modal>
  )
}
