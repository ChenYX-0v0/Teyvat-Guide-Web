import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { Compass } from 'lucide-react'
import clsx from 'clsx'
import { useAuthStore } from '@/store/useAuthStore'
import { t } from '@/i18n'

/**
 * 登录页（邀请制）。
 *
 * 本站当前是**不公开部署**：主服务 `auth.allow_register=false`，`POST /api/auth/register` 直接 403，
 * 因此这里不再提供注册入口——账号由管理员在后台「用户管理 → 新增用户」创建后转达。
 * 口径见 shared-docs/04 §2.3、web/11 §7.6；将来若改为公开，把注册模式加回来即可
 * （i18n 的 `login.register*` 等文案与 `useAuthStore.register` 都还在）。
 */
export default function LoginPage() {
  const navigate = useNavigate()
  const login = useAuthStore((s) => s.login)

  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (!username.trim() || !password) return
    setBusy(true)
    setError('')
    try {
      await login(username.trim(), password)
      navigate('/', { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : t('errors.unknown'))
    } finally {
      setBusy(false)
    }
  }

  // 输入框字号移动端 ≥16px：iOS 聚焦更小字号会自动放大页面（sm 起回到 text-sm）
  const inputClass =
    'w-full rounded-xl border border-white/10 bg-white/5 px-4 py-2.5 text-base outline-none transition placeholder:text-white/25 focus:border-teyvat-gold/50 sm:text-sm'

  return (
    <div className="flex h-full items-center justify-center px-4">
      <div className="w-full max-w-sm rounded-2xl border border-white/10 bg-teyvat-panel/60 p-7 shadow-2xl">
        <div className="flex flex-col items-center gap-2 text-center">
          <Compass className="h-9 w-9 text-teyvat-gold" />
          <h1 className="text-lg font-semibold text-teyvat-gold">{t('header.title')}</h1>
          <p className="text-xs text-white/40">{t('login.welcome')}</p>
        </div>

        <p className="mt-4 rounded-xl border border-teyvat-gold/20 bg-teyvat-gold/5 px-3 py-2 text-center text-[11px] leading-relaxed text-white/55">
          {t('login.inviteOnly')}
          <br />
          {t('login.inviteHint')}
        </p>

        <form onSubmit={submit} className="mt-5 space-y-3">
          <input
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            placeholder={t('login.username')}
            autoFocus
            className={inputClass}
          />
          <input
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            type="password"
            placeholder={t('login.password')}
            className={inputClass}
          />

          {error && (
            <p className="rounded-lg border border-red-400/30 bg-red-400/10 px-3 py-2 text-xs text-red-300">
              {error}
            </p>
          )}

          <button
            type="submit"
            disabled={busy || !username.trim() || !password}
            className={clsx(
              'w-full rounded-xl bg-teyvat-gold py-2.5 text-sm font-medium text-teyvat-ink transition',
              'hover:bg-teyvat-gold-deep disabled:cursor-not-allowed disabled:opacity-40',
            )}
          >
            {busy ? t('common.loading') : t('login.submit')}
          </button>
        </form>
      </div>
    </div>
  )
}
