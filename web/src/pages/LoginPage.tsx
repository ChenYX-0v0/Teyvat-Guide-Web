import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { Compass } from 'lucide-react'
import Card from '@/components/common/Card'
import Button from '@/components/common/Button'
import SiteFooter from '@/components/layout/SiteFooter'
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
    'w-full rounded-xl border border-teyvat-line bg-white/[0.04] px-4 py-2.5 text-base text-teyvat-text outline-none transition placeholder:text-teyvat-text-4 focus:border-teyvat-line-strong focus:shadow-glow sm:text-sm'

  return (
    // pb-10：给底部固定的备案号页脚留位，避免短屏时卡片与页脚重叠
    <div className="flex h-full items-center justify-center px-4 pb-10">
      <Card className="relative w-full max-w-sm overflow-hidden p-7">
        {/* 顶部金色微光：给登录卡一点"仪式感"，不做花哨装饰 */}
        <span
          aria-hidden
          className="pointer-events-none absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-teyvat-gold/60 to-transparent"
        />

        <div className="flex flex-col items-center gap-2 text-center">
          <span className="flex h-12 w-12 items-center justify-center rounded-2xl border border-teyvat-line-strong bg-teyvat-gold/10">
            <Compass className="h-6 w-6 text-teyvat-gold" />
          </span>
          <h1 className="text-lg font-semibold tracking-wide text-teyvat-gold">
            {t('header.title')}
          </h1>
          <p className="text-xs text-teyvat-text-3">{t('login.welcome')}</p>
        </div>

        <p className="mt-4 rounded-xl border border-teyvat-line-strong bg-teyvat-gold/5 px-3 py-2 text-center text-[11px] leading-relaxed text-teyvat-text-3">
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
            <p className="rounded-lg border border-err/30 bg-err/10 px-3 py-2 text-xs text-err">
              {error}
            </p>
          )}

          <Button
            type="submit"
            variant="primary"
            className="w-full"
            disabled={busy || !username.trim() || !password}
          >
            {busy ? t('common.loading') : t('login.submit')}
          </Button>
        </form>
      </Card>
      {/* 登录页在 AppLayout 之外：备案号必须自带（公安审核的第一屏很可能就是这里，见 web/11 §6.5） */}
      <SiteFooter className="fixed inset-x-0 bottom-0" />
    </div>
  )
}
