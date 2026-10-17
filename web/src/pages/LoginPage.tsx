import { useEffect, useState, type FormEvent } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { Compass, Sparkles } from 'lucide-react'
import Card from '@/components/common/Card'
import Alert from '@/components/common/Alert'
import Button from '@/components/common/Button'
import SiteFooter from '@/components/layout/SiteFooter'
import { ApiError } from '@/api/client'
import { authApi } from '@/api/auth'
import { useAuthStore } from '@/store/useAuthStore'
import { t } from '@/i18n'

/**
 * 登录 / 注册页。
 *
 * **注册口径（2026-09-27 起）**：手机号 + 短信验证码 + 设置密码，注册成功后自动登录。
 * 账号名就是手机号 ⇒ 登录链路不用改（后端 `service/auth.go` 的约定 ✓）。
 *
 * ⚠️ 注册入口**不能硬编码** ✗：是否显示取决于服务端 `GET /api/auth/config`
 * （`allowRegister` + `smsEnabled`）。不公开部署（当前线上口径）下两者都是 false，
 * 页面退回"邀请制"提示 ✓；硬编码会出现"点了注册才发现 403"的死按钮 ✗。
 */
const PHONE_RE = /^1[3-9]\d{9}$/

export default function LoginPage() {
  const navigate = useNavigate()
  const login = useAuthStore((s) => s.login)
  const register = useAuthStore((s) => s.register)
  const registerOptions = useAuthStore((s) => s.registerOptions)
  const loadRegisterOptions = useAuthStore((s) => s.loadRegisterOptions)

  const [mode, setMode] = useState<'login' | 'register'>('login')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  // 邀请码：**放在手机号上方**（用户 2026-09-28 口径 ✓）—— 它是"准入"，手机号是"身份"
  const [inviteCode, setInviteCode] = useState('')
  const [phone, setPhone] = useState('')
  const [code, setCode] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [busy, setBusy] = useState(false)
  const [sending, setSending] = useState(false)
  const [countdown, setCountdown] = useState(0)

  // 注册可用性：进页面拉一次（失败按"不可注册"处理 ✓ —— 登录不受影响）
  useEffect(() => {
    void loadRegisterOptions()
  }, [loadRegisterOptions])

  // 重发倒计时：用 setTimeout 递归而不是 setInterval（依赖闭合更清楚 ✓）
  useEffect(() => {
    if (countdown <= 0) return
    const timer = window.setTimeout(() => setCountdown((n) => n - 1), 1000)
    return () => window.clearTimeout(timer)
  }, [countdown])

  const canRegister = Boolean(registerOptions?.allowRegister && registerOptions?.smsEnabled)
  // 是否要求邀请码：**由服务端配置决定，不硬编码** ✓（require_invite_code=false 时这一栏直接不显示）
  const requireInvite = Boolean(registerOptions?.requireInviteCode)
  // 只要求"非空"：格式与是否有效**交给服务端**判定 ✓（前端再写一套正则，改规则时就会两边漂移 ✗）
  const inviteOk = !requireInvite || inviteCode.trim().length > 0
  const phoneValid = PHONE_RE.test(phone.trim())
  const codeValid = /^\d{4,8}$/.test(code.trim())
  const passValid = password.length >= 6

  const switchMode = (next: 'login' | 'register') => {
    setMode(next)
    setError('')
    setNotice('')
  }

  const submitLogin = async (e: FormEvent) => {
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

  const sendCode = async () => {
    // ⚠️ 邀请码不填 / 不合法 ⇒ **按钮就是禁用的** ✓（服务端也会再拦一道 ✓）
    if (!phoneValid || !inviteOk || sending || countdown > 0) return
    setSending(true)
    setError('')
    setNotice('')
    try {
      const res = await authApi.sendSmsCode(phone.trim(), inviteCode.trim() || undefined)
      setCountdown(res.resendAfterSeconds || 60)
      setNotice(registerOptions?.smsMock ? t('login.codeSentMock') : t('login.codeSent'))
    } catch (err) {
      setError(err instanceof Error ? err.message : t('errors.unknown'))
      // 429 里带 retryAfterSeconds：直接把倒计时接上（不然用户只能瞎猜要等多久 ✗）
      const retry = err instanceof ApiError ? Number((err.data as { retryAfterSeconds?: number } | null)?.retryAfterSeconds) : NaN
      if (Number.isFinite(retry) && retry > 0) setCountdown(retry)
    } finally {
      setSending(false)
    }
  }

  const submitRegister = async (e: FormEvent) => {
    e.preventDefault()
    if (!phoneValid || !codeValid || !passValid || !inviteOk) return
    setBusy(true)
    setError('')
    setNotice('')
    try {
      await register({
        phone: phone.trim(),
        code: code.trim(),
        password,
        displayName: displayName.trim() || undefined,
        inviteCode: inviteCode.trim() || undefined,
      })
      navigate('/', { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : t('errors.unknown'))
    } finally {
      setBusy(false)
    }
  }

  // 输入框字号移动端 ≥16px：iOS 聚焦更小字号会自动放大页面（sm 起回到 text-sm）
  const inputClass =
    'w-full rounded-xl border border-teyvat-line bg-teyvat-fill px-4 py-2.5 text-base text-teyvat-text outline-none transition tx-ease tx-fast placeholder:text-teyvat-text-4 hover:border-teyvat-line-strong focus:border-teyvat-line-strong focus:shadow-glow sm:text-sm'
  const registering = mode === 'register' && canRegister

  return (
    // pb-10：给底部固定的备案号页脚留位，避免短屏时卡片与页脚重叠
    <div className="flex h-full items-center justify-center px-4 pb-10">
      <Card className="tx-enter relative w-full max-w-sm overflow-hidden p-7">
        {/* 顶部金色微光：给登录卡一点"仪式感"，不做花哨装饰 */}
        <span
          aria-hidden
          className="pointer-events-none absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-teyvat-gold/60 to-transparent"
        />

        <div className="flex flex-col items-center gap-3 text-center">
          <span className="flex h-12 w-12 items-center justify-center rounded-xl border border-teyvat-line-strong bg-teyvat-gold/10 text-teyvat-gold">
            <Compass className="h-6 w-6" />
          </span>
          <div>
            <h1 className="text-[20px] leading-tight font-semibold tracking-[0.02em] text-teyvat-gold">
              {t('header.title')}
            </h1>
            {/* 2026-10-11：注册模式不再重复一句「创建你的旅行者账号」（上方标题已经说明身份 ✓），
                这个位置只在登录态显示欢迎语 ✓；注册须知统一放到下面的表单顶部 ✓ */}
            {!registering && (
              <p className="mt-1.5 text-xs text-teyvat-text-4">{t('login.welcome')}</p>
            )}
          </div>
        </div>

        {/* 注册关闭时的口径说明（服务端 allow_register=false / 短信未开启）*/}
        {!canRegister && (
          <Alert tone="info" className="mt-4 justify-center text-center">
            {t('login.inviteOnly')}
            <br />
            {t('login.inviteHint')}
          </Alert>
        )}

        {registering ? (
          <form onSubmit={submitRegister} className="mt-5 space-y-3">
            {/* 注册须知（普通小字 ✓，**不要套框** ✗）—— 2026-10-11：原「用手机号注册，注册后可直接用手机号登录」
                已去掉，改为说明"需要一次性邀请码"；用户反馈套框反而不好看 ⇒ 就一行小字 ✓
                （与原来那行同样式：text-[11px] + text-teyvat-text-4 ✓，左对齐、无边框无底色 ✓）。
                只在服务端确实要求邀请码时显示（requireInvite ✓），否则会出现"要求邀请码但其实没开"的错提示 ✗。 */}
            {requireInvite && (
              <p className="text-[11px] leading-relaxed text-teyvat-text-4">{t('login.inviteCodeNotice')}</p>
            )}

            {/* 邀请码在**手机号上方** ✓：不填就拿不到验证码（服务端同样会拦 ✓） */}
            {requireInvite && (
              <input
                value={inviteCode}
                onChange={(e) => setInviteCode(e.target.value)}
                placeholder={t('login.inviteCode')}
                autoComplete="off"
                autoFocus
                className={inputClass}
              />
            )}

            <div className="flex gap-2">
              <input
                value={phone}
                onChange={(e) => setPhone(e.target.value)}
                placeholder={t('login.phone')}
                inputMode="numeric"
                autoComplete="tel"
                autoFocus={!requireInvite}
                className={inputClass}
              />
              <Button
                type="button"
                variant="soft"
                size="sm"
                className="shrink-0"
                disabled={!phoneValid || !inviteOk || sending || countdown > 0}
                onClick={sendCode}
              >
                {sending
                  ? t('login.sendingCode')
                  : countdown > 0
                    ? t('login.resendIn', { seconds: countdown })
                    : t('login.sendCode')}
              </Button>
            </div>

            <input
              value={code}
              onChange={(e) => setCode(e.target.value)}
              placeholder={t('login.smsCode')}
              inputMode="numeric"
              autoComplete="one-time-code"
              className={inputClass}
            />
            <input
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              type="password"
              placeholder={t('login.password')}
              autoComplete="new-password"
              className={inputClass}
            />
            <input
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              placeholder={t('login.displayName')}
              className={inputClass}
            />
            {!passValid && password.length > 0 && (
              <p className="text-[11px] text-teyvat-text-4">{t('login.passwordRule')}</p>
            )}

            {error && <Alert tone="error">{error}</Alert>}
            {notice && !error && <Alert tone="info">{notice}</Alert>}

            <Button
              type="submit"
              variant="primary"
              className="w-full"
              disabled={busy || !phoneValid || !codeValid || !passValid || !inviteOk}
            >
              {busy ? t('common.loading') : t('login.registerSubmit')}
            </Button>

            <p className="pt-1 text-center text-xs text-teyvat-text-3">
              {t('login.hasAccount')}{' '}
              <button
                type="button"
                className="text-teyvat-gold transition hover:underline"
                onClick={() => switchMode('login')}
              >
                {t('login.switchToLogin')}
              </button>
            </p>
          </form>
        ) : (
          <form onSubmit={submitLogin} className="mt-5 space-y-3">
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
              autoComplete="current-password"
              className={inputClass}
            />

            {error && <Alert tone="error">{error}</Alert>}

            <Button
              type="submit"
              variant="primary"
              className="w-full"
              disabled={busy || !username.trim() || !password}
            >
              {busy ? t('common.loading') : t('login.submit')}
            </Button>

            {canRegister && (
              <p className="pt-1 text-center text-xs text-teyvat-text-3">
                {t('login.noAccount')}{' '}
                <button
                  type="button"
                  className="text-teyvat-gold transition hover:underline"
                  onClick={() => switchMode('register')}
                >
                  {t('login.switchToRegister')}
                </button>
              </p>
            )}
          </form>
        )}

        {/* 演示入口（2026-10-17）：不想注册的人也能先看"档案 + 派蒙分析"长什么样 ✓。
            演示页是**公开路由**（/demo），内容全部预置，不调模型、不消耗 token ✓ */}
        <div className="mt-4 border-t border-teyvat-line pt-4 text-center">
          <Link
            to="/demo"
            className="inline-flex items-center gap-1.5 text-xs text-teyvat-text-3 transition hover:text-teyvat-gold"
          >
            <Sparkles className="h-3.5 w-3.5" />
            {t('demo.loginEntry')}
          </Link>
        </div>
      </Card>
      {/* 登录页在 AppLayout 之外：备案号必须自带（公安审核的第一屏很可能就是这里，见 web/11 §6.5） */}
      <SiteFooter className="fixed inset-x-0 bottom-0" />
    </div>
  )
}
