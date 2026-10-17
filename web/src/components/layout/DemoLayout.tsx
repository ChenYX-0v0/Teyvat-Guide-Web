import { Link, Outlet } from 'react-router-dom'
import AppShell from './AppShell'
import Chip from '@/components/common/Chip'
import { demoNavItems } from './navItems'
import { buttonClass } from '@/utils/buttonStyles'
import { t } from '@/i18n'

/**
 * 演示模式外壳（2026-10-17 改版，见 TODO-32 ✓）—— **公开**，不含任何鉴权。
 *
 * 早先这里是自己写的一套"顶栏 ＋ 顶部页签"✗，与登录区的侧栏 / 抽屉形态不一致，
 * 访客先看演示页、登录后看真实页会像两个产品 ✗。现在直接复用登录区同一个 `AppShell` ✓，
 * 差异只剩**必须不同**的三处 ✓：
 *   1. 标题旁多一个「演示」徽标（合规：让访客知道这不是真人账号 ✓）；
 *   2. 侧栏只有演示两项（登录区那几项对匿名用户点进去都会被踢回登录页 ✗）；
 *   3. 侧栏底部是"说明 ＋ 登录入口 ＋ 用户须知"，而不是用户卡片（没有账号可管 ✓）。
 *
 * ⚠️ 仍然**不复用 `AppLayout`** ✗：那层会跑 `verify()` 校验 Token、并弹强制阅读的用户须知，
 * 游客一进来就被踢走 ✓（见 `AppShell` 头注释）。
 */
export default function DemoLayout() {
  return (
    <AppShell
      navItems={demoNavItems}
      headerExtra={<Chip tone="gold">{t('demo.badge')}</Chip>}
      aside={() => <DemoAside />}
    >
      <Outlet />
    </AppShell>
  )
}

/** 侧栏 / 抽屉底部区块：与登录区的 `UserCard` 同位同宽（`mt-auto` 压到底部 ✓） */
function DemoAside() {
  return (
    <div className="mt-auto flex flex-col gap-2 rounded-xl border border-teyvat-line bg-teyvat-fill p-3">
      <p className="text-[11px] leading-relaxed text-teyvat-text-4">{t('demo.asideHint')}</p>
      <Link
        to="/login"
        className={buttonClass({ variant: 'primary', size: 'sm' }) + ' justify-center'}
      >
        {t('demo.loginCta')}
      </Link>
      <Link
        to="/terms"
        className="text-center text-[11px] text-teyvat-text-3 transition tx-ease tx-fast hover:text-teyvat-gold"
      >
        {t('demo.asideTerms')}
      </Link>
    </div>
  )
}
