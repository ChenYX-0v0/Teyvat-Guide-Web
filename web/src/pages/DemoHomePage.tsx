import { Link } from 'react-router-dom'
import { Info } from 'lucide-react'
import HomeView from '@/components/home/HomeView'
import { buttonClass } from '@/utils/buttonStyles'
import { t } from '@/i18n'

/**
 * 演示首页（2026-10-17，公开路由 `/demo` 的 index —— 与登录区 `/` 同一张首页 ✓）。
 *
 * 正文直接复用 `components/home/HomeView` ✓，差异只有三处（都该不同 ✓）：
 *  1. 主 CTA 指向**演示对话**（`/demo/chat`），而不是需要登录的 `/chat` ✓；
 *  2. 次按钮是「登录 / 注册」——演示区唯一的转化动作 ✓；
 *  3. 状态行下面多一句演示说明（合规：让访客知道内容是预置示例 ✓）。
 * 后端状态一行是**真实**健康检查（公开接口 ✓），不搞假绿灯 ✗。
 */
export default function DemoHomePage() {
  return (
    <HomeView
      ctaTo="/demo/chat"
      ctaLabel={t('demo.homeCta')}
      extraCta={
        <Link to="/login" className={buttonClass({ variant: 'ghost' })}>
          {t('demo.loginCta')}
        </Link>
      }
      hint={
        <p className="mt-4 flex items-start gap-2 rounded-2xl border border-teyvat-line-strong bg-teyvat-gold/5 p-3 text-[11px] leading-relaxed text-teyvat-text-2">
          <Info className="mt-0.5 h-3.5 w-3.5 shrink-0 text-teyvat-gold" />
          <span>{t('demo.homeHint')}</span>
        </p>
      }
    />
  )
}
