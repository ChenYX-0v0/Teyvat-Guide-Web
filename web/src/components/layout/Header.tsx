import { Compass, Menu } from 'lucide-react'
import { t } from '@/i18n'

type Props = {
  /** 移动端汉堡按钮：由 AppLayout 控制抽屉开关 */
  onMenuClick: () => void
}

/**
 * 顶部栏：**只有品牌与移动端菜单按钮** ✓。
 *
 * 2026-10-11：原先右上角的「昵称胶囊 + 退出登录图标」已按用户要求**移到侧栏最底部**
 * （`layout/UserCard` ✓，桌面侧栏与移动抽屉共用同一块卡片 ✓）——
 * 头像/昵称/个人信息/改密码/退出登录聚在一处，顶部栏不再散着账号入口 ✓。
 */
export default function Header({ onMenuClick }: Props) {
  return (
    <header className="flex items-center justify-between gap-3 border-b border-teyvat-line bg-black/20 px-4 py-3 backdrop-blur sm:px-6">
      <div className="flex min-w-0 items-center gap-2 sm:gap-3">
        <button
          type="button"
          onClick={onMenuClick}
          aria-label={t('header.menu')}
          className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border border-teyvat-line text-teyvat-text-2 transition hover:border-teyvat-line-strong hover:text-teyvat-text md:hidden"
        >
          <Menu className="h-4 w-4" />
        </button>
        <Compass className="h-6 w-6 shrink-0 text-teyvat-gold" />
        <div className="min-w-0">
          <h1 className="truncate text-base font-semibold tracking-wide text-teyvat-gold sm:text-lg">
            {t('header.title')}
          </h1>
          {/* 副标题为空时不渲染（英文界面下标题已是 Teyvat Guide，无需重复） */}
          {t('header.subtitle') ? (
            <p className="hidden truncate text-xs text-teyvat-text-3 sm:block">{t('header.subtitle')}</p>
          ) : null}
        </div>
      </div>

    </header>
  )
}
