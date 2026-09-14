import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { Compass } from 'lucide-react'
import clsx from 'clsx'
import { useAuthStore } from '@/store/useAuthStore'
import { t } from '@/i18n'

type Mode = 'login' | 'register'

export default function LoginPage() {
  const navigate = useNavigate()
  const login = useAuthStore((s) => s.login)
  const register = useAuthStore((s) => s.register)

  const [mode, setMode] = useState<Mode>('login')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (!username.trim() || !password) return
    setBusy(true)
    setError('')
    try {
      if (mode === 'login') {
        await login(username.trim(), password)
      } else {
        await register(username.trim(), password, displayName.trim() || undefined)
      }
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
          <p className="text-xs text-white/40">
            {mode === 'login' ? t('login.welcome') : t('login.registerWelcome')}
          </p>
        </div>

        <form onSubmit={submit} className="mt-6 space-y-3">
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
          {mode === 'register' && (
            <input
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              placeholder={t('login.displayName')}
              className={inputClass}
            />
          )}

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
            {busy
              ? t('common.loading')
              : mode === 'login'
                ? t('login.submit')
                : t('login.registerSubmit')}
          </button>
        </form>

        <p className="mt-4 text-center text-xs text-white/45">
          {mode === 'login' ? (
            <>
              {t('login.noAccount')}{' '}
              <button
                type="button"
                onClick={() => {
                  setMode('register')
                  setError('')
                }}
                className="text-teyvat-gold hover:underline"
              >
                {t('login.switchToRegister')}
              </button>
            </>
          ) : (
            <>
              {t('login.hasAccount')}{' '}
              <button
                type="button"
                onClick={() => {
                  setMode('login')
                  setError('')
                }}
                className="text-teyvat-gold hover:underline"
              >
                {t('login.switchToLogin')}
              </button>
            </>
          )}
        </p>
      </div>
    </div>
  )
}
