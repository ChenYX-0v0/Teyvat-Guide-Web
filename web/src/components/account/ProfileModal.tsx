import { useEffect, useState, type FormEvent } from 'react'
import Modal from '@/components/common/Modal'
import Button from '@/components/common/Button'
import { authApi, type MeResult } from '@/api/auth'
import { useAuthStore } from '@/store/useAuthStore'
import { t } from '@/i18n'

type Props = {
  open: boolean
  onClose: () => void
}

/**
 * 个人信息管理（侧栏用户卡片的第一个功能，2026-10-11）。
 *
 * 能改的只有**昵称** ✓ —— 账号（手机号）是登录凭据，不可改 ✗；
 * 游戏 UID 绑定是账号页的活儿，这里只**显示** ✓（不在两个地方各做一套 ✗）。
 *
 * 2026-10-11 按用户要求**精简**：标签「用户名（登录用）」→「账号」✓、去掉昵称下方的说明小字 ✓、
 * 去掉底部「去账号信息页 / 查看用户须知」两个按钮 ✓
 * —— 角色绑定与条款在左侧导航与页脚另有入口 ✓，不必在这个小弹窗里再兜一圈 ✗。
 * 所以它现在只干两件事：**看一眼账号信息 + 改昵称** ✓。
 *
 * ⚠️ 账号与绑定 UID **不在 auth store 里**，所以打开时拉一次 `/auth/me` ✓：
 * 这次请求很轻，为它去扩全局 state 不划算 ✓（而且拉到的总是最新值 ✓）。
 */
export default function ProfileModal({ open, onClose }: Props) {
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
    'mt-1 w-full rounded-xl border border-teyvat-line bg-teyvat-fill px-3 py-2 text-base text-teyvat-text outline-none transition placeholder:text-teyvat-text-4 focus:border-teyvat-line-strong focus:shadow-glow sm:text-sm'

  return (
    <Modal
      open={open}
      title={t('profile.title')}
      onClose={onClose}
      footer={
        // Modal 本身不带关闭按钮（见 common/Modal.tsx），所以这里保留「取消」✓：
        // 否则界面上只剩一个「保存」，只能靠 Esc / 点遮罩关闭，不好发现 ✗。
        <div className="flex items-center justify-end gap-2">
          <Button variant="ghost" size="sm" onClick={onClose} disabled={busy}>
            {t('common.cancel')}
          </Button>
          <Button variant="primary" type="submit" form="profile-form" disabled={busy || !loaded}>
            {t('common.save')}
          </Button>
        </div>
      }
    >
      <form id="profile-form" onSubmit={save} className="space-y-4">
        <div>
          <span className="text-xs text-teyvat-text-3">{t('profile.account')}</span>
          <p className="mt-1 truncate rounded-xl border border-teyvat-line bg-teyvat-fill px-3 py-2 text-sm text-teyvat-text-2">
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
            // （placeholder 用账号兜底：留空即回落显示账号 ✓，不再多写一行说明小字 ✓）
            maxLength={40}
            placeholder={me?.username ?? ''}
            className={inputClass}
          />
        </div>

        <div>
          <span className="text-xs text-teyvat-text-3">{t('profile.uidLabel')}</span>
          <p className="mt-1 truncate rounded-xl border border-teyvat-line bg-teyvat-fill px-3 py-2 text-sm text-teyvat-text-2">
            {me?.genshinUid ? me.genshinUid : t('profile.notBound')}
          </p>
        </div>

        {error ? <p className="text-xs text-err">{error}</p> : null}
        {saved ? <p className="text-xs text-ok">{t('profile.saved')}</p> : null}
      </form>
    </Modal>
  )
}
