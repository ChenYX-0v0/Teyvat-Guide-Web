import { Link } from 'react-router-dom'
import { ArrowLeft } from 'lucide-react'
import TermsContent from '@/components/legal/TermsContent'
import SiteFooter from '@/components/layout/SiteFooter'
import { useAuthStore } from '@/store/useAuthStore'
import { t } from '@/i18n'

/**
 * 用户须知（完整条款）——**公开页面，不需要登录**。
 *
 * 为什么单独做公开页而不是只放在弹窗里：
 *  1. 未登录用户 / 审核人员打开站点就能直接读到（不依赖登录、不依赖弹窗）；
 *  2. 弹窗受"阅读 5 秒"约束，公开页可以慢慢看；
 *  3. 页脚有常驻入口，条款变更后老链接也有效。
 */
export default function TermsPage() {
  // 版本号由服务端下发（登录后才有）；未登录时只展示正文，不展示可能过期的版本号
  const version = useAuthStore((s) => s.termsCurrentVersion)

  return (
    <div className="flex min-h-dvh flex-col">
      <div className="mx-auto w-full max-w-3xl flex-1 px-4 py-8 sm:px-6">
        <Link
          to="/"
          className="inline-flex items-center gap-1.5 text-xs text-teyvat-text-3 transition-colors hover:text-teyvat-text-2"
        >
          <ArrowLeft className="h-3.5 w-3.5" />
          {t('terms.back')}
        </Link>
        <h1 className="mt-4 text-lg font-semibold tracking-wide text-teyvat-gold">{t('terms.title')}</h1>
        <div className="mt-4">
          <TermsContent version={version || undefined} />
        </div>
      </div>
      <SiteFooter />
    </div>
  )
}
