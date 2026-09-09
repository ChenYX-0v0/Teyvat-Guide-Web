import { t } from '@/i18n'

const placeholder = ['钟离', '纳西妲', '芙宁娜', '枫原万叶']

export default function WikiPage() {
  return (
    <div className="mx-auto max-w-4xl px-6 py-14">
      <h2 className="text-2xl font-semibold text-teyvat-gold">{t('wiki.title')}</h2>
      <p className="mt-2 text-sm text-white/55">{t('wiki.placeholder')}</p>
      <div className="mt-8 grid gap-3 sm:grid-cols-4">
        {placeholder.map((name) => (
          <div
            key={name}
            className="rounded-xl border border-white/10 bg-teyvat-panel/60 px-4 py-6 text-center text-sm text-white/70"
          >
            {name}
          </div>
        ))}
      </div>
    </div>
  )
}
