import { useEffect, useState, type FormEvent } from 'react'
import Modal from '@/components/common/Modal'
import Button from '@/components/common/Button'
import { useAuthStore } from '@/store/useAuthStore'
import { t } from '@/i18n'

type Props = {
  open: boolean
  onClose: () => void
}

/**
 * 修改密码（侧栏用户卡片的第二个功能，2026-10-11）。
 *
 * 前端只做**便宜且确定**的两项检查：长度 ≥6 ✓ 与"两次输入一致" ✓（纯 UX ✓）；
 * "当前密码对不对"**只能由服务端判** ✓ —— 而且服务端刻意用 400 而不是 401 ✓，
 * 所以我们这里是普通的错误提示，**不会把人踢到登录页** ✓（契约见 handler/auth.go）。
 *
 * ⚠️ 成功后**不重新登录** ✓：服务端不吊销既有 Token（JWT 里没有密码版本位 ✓），当前设备继续可用 ✓。
 *
 * 2026-10-11 按用户要求：**去掉**了原先那行"其他设备不会被自动退出…"的小字提示 ✗。
 * ⚠️ 去掉的只是**界面提示** —— **后端行为一点没变**（改密仍不影响其他设备的 Token ✓）：
 * 这条边界仍写在 `web/04` §3 与 `shared/04` §2.6 里 ✓，别因为界面不提醒就以为已经踢下线了 ✗。
 */
export default function PasswordModal({ open, onClose }: Props) {
  const changePassword = useAuthStore((s) => s.changePassword)

  const [oldPwd, setOldPwd] = useState('')
  const [newPwd, setNewPwd] = useState('')
  const [confirmPwd, setConfirmPwd] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [ok, setOk] = useState(false)

  // 每次打开都清空：密码不该留在输入框里（关掉再打开是全新一次 ✓）
  useEffect(() => {
    if (!open) return
    setOldPwd('')
    setNewPwd('')
    setConfirmPwd('')
    setError('')
    setOk(false)
  }, [open])

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (newPwd.length < 6) {
      setError(t('login.passwordRule'))
      return
    }
    if (newPwd !== confirmPwd) {
      setError(t('password.mismatch'))
      return
    }
    setBusy(true)
    setError('')
    setOk(false)
    try {
      await changePassword(oldPwd, newPwd)
      setOk(true)
      setOldPwd('')
      setNewPwd('')
      setConfirmPwd('')
    } catch (err) {
      setError(err instanceof Error ? err.message : t('errors.unknown'))
    } finally {
      setBusy(false)
    }
  }

  const inputClass =
    'mt-1 w-full rounded-xl border border-teyvat-line bg-teyvat-fill px-3 py-2 text-base text-teyvat-text outline-none transition placeholder:text-teyvat-text-4 focus:border-teyvat-line-strong focus:shadow-glow sm:text-sm'

  return (
    <Modal
      open={open}
      title={t('password.title')}
      onClose={onClose}
      footer={
        <div className="flex items-center justify-end gap-2">
          <Button variant="ghost" size="sm" onClick={onClose} disabled={busy}>
            {t('common.cancel')}
          </Button>
          <Button variant="primary" type="submit" form="password-form" disabled={busy}>
            {t('password.submit')}
          </Button>
        </div>
      }
    >
      <form id="password-form" onSubmit={submit} className="space-y-4">
        <div>
          <label htmlFor="pwd-old" className="text-xs text-teyvat-text-3">
            {t('password.current')}
          </label>
          <input
            id="pwd-old"
            type="password"
            autoComplete="current-password"
            value={oldPwd}
            onChange={(e) => setOldPwd(e.target.value)}
            className={inputClass}
          />
        </div>

        <div>
          <label htmlFor="pwd-new" className="text-xs text-teyvat-text-3">
            {t('password.new')}
          </label>
          <input
            id="pwd-new"
            type="password"
            autoComplete="new-password"
            value={newPwd}
            onChange={(e) => setNewPwd(e.target.value)}
            className={inputClass}
          />
          <p className="mt-1 text-[11px] leading-relaxed text-teyvat-text-4">{t('login.passwordRule')}</p>
        </div>

        <div>
          <label htmlFor="pwd-confirm" className="text-xs text-teyvat-text-3">
            {t('password.confirm')}
          </label>
          <input
            id="pwd-confirm"
            type="password"
            autoComplete="new-password"
            value={confirmPwd}
            onChange={(e) => setConfirmPwd(e.target.value)}
            className={inputClass}
          />
        </div>

        {error ? <p className="text-xs text-err">{error}</p> : null}
        {ok ? <p className="text-xs text-ok">{t('password.ok')}</p> : null}
      </form>
    </Modal>
  )
}
