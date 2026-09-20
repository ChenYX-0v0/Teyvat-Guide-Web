import Card from '@/components/common/Card'
import { elementColor } from '@/utils/elementColor'
import { t } from '@/i18n'

/** 占位数据（真实数据接入见 docs/12 P2-2） */
const placeholder = [
  { name: '钟离', element: '岩' },
  { name: '纳西妲', element: '草' },
  { name: '芙宁娜', element: '水' },
  { name: '枫原万叶', element: '风' },
]

export default function WikiPage() {
  return (
    <div className="mx-auto max-w-4xl px-6 py-14">
      <h2 className="text-2xl font-semibold text-teyvat-gold">{t('wiki.title')}</h2>
      <p className="mt-2 text-sm text-teyvat-text-3">{t('wiki.placeholder')}</p>
      <div className="mt-8 grid gap-3 sm:grid-cols-4">
        {placeholder.map(({ name, element }) => (
          <Card key={name} lift className="px-4 py-6 text-center">
            <span
              className="mx-auto mb-2 block h-2.5 w-2.5 rounded-full"
              style={{ background: elementColor(element) }}
            />
            <span className="text-sm text-teyvat-text-2">{name}</span>
          </Card>
        ))}
      </div>
    </div>
  )
}
