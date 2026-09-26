import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import Modal from '@/components/common/Modal'
import Button from '@/components/common/Button'
import TermsContent from './TermsContent'
import { useAuthStore } from '@/store/useAuthStore'
import { EVENTS, track } from '@/lib/track'
import { t } from '@/i18n'

/**
 * 强制阅读秒数：在此之前的"我已阅读并同意"按钮保持禁用。
 * 目的不是防机器（前端挡不住伪造请求），而是**确保人真的有机会读完**——
 * 监管口径要的正是"已向用户明示并取得确认"，服务端有版本留痕（users.terms_version）。
 */
const READ_SECONDS = 5

/**
 * 用户须知强制确认弹窗（阻断式：不可关闭、不可绕过）。
 *
 * 触发条件由 `AppLayout` 判断：`termsCurrentVersion !== termsAcceptedVersion`。
 * 版本真源在主服务（`server/internal/service/terms.go`），文案改动会递增版本号，
 * 于是**所有用户都会被重新要求确认一次**。
 */
export default function TermsModal({ version }: { version: string }) {
  const navigate = useNavigate()
  const acceptTerms = useAuthStore((s) => s.acceptTerms)
  const logout = useAuthStore((s) => s.logout)

  const [left, setLeft] = useState(READ_SECONDS)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    track(EVENTS.termsShown, { version })
    const timer = window.setInterval(() => setLeft((n) => (n <= 1 ? 0 : n - 1)), 1000)
    return () => window.clearInterval(timer)
  }, [version])

  const agree = async () => {
    setBusy(true)
    setError('')
    try {
      await acceptTerms()
      track(EVENTS.termsAccepted, { version })
    } catch (e) {
      setError(e instanceof Error ? e.message : t('errors.unknown'))
    } finally {
      setBusy(false)
    }
  }

  const decline = () => {
    logout()
    navigate('/login', { replace: true })
  }

  return (
    <Modal
      open
      blocking
      title={t('terms.title')}
      footer={
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
          <p className="text-[11px] leading-relaxed text-teyvat-text-3">
            {left > 0 ? t('terms.waitSeconds', { seconds: left }) : t('terms.readyHint')}
          </p>
          <div className="flex shrink-0 items-center gap-2">
            <Button variant="ghost" onClick={decline} disabled={busy}>
              {t('terms.decline')}
            </Button>
            <Button variant="primary" onClick={agree} disabled={busy || left > 0}>
              {left > 0 ? t('terms.agreeCountdown', { seconds: left }) : t('terms.agree')}
            </Button>
          </div>
        </div>
      }
    >
      <TermsContent version={version} />
      {error ? <p className="mt-3 text-xs text-err">{error}</p> : null}
    </Modal>
  )
}
