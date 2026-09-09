import { useAuthStore } from '@/store/useAuthStore'
import { t } from '@/i18n'

export default function AdminPage() {
  const displayName = useAuthStore((s) => s.displayName)

  return (
    <div className="space-y-6">
      <div className="rounded-2xl border border-teyvat-gold/20 bg-teyvat-gold/5 p-6">
        <h1 className="text-lg font-semibold text-teyvat-gold">
          {t('admin.title')}
        </h1>
        <p className="mt-1 text-sm text-white/55">
          {t('admin.welcome', { name: displayName })}
        </p>
      </div>
      {/* 管理功能占位：用户管理 / 资产导入 / 缺漏补录等后续在此扩展 */}
    </div>
  )
}
