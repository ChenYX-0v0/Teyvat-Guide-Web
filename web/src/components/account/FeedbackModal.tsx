import { useEffect, useState, type FormEvent } from 'react'
import Modal from '@/components/common/Modal'
import Button from '@/components/common/Button'
import { feedbackApi, type FeedbackCategory } from '@/api/feedback'
import { t } from '@/i18n'

type Props = {
  open: boolean
  onClose: () => void
}

/** 类别选项：与服务端白名单一一对应（`server/internal/service/feedback.go` ✓） */
const CATEGORIES: FeedbackCategory[] = ['bug', 'suggest', 'experience', 'other']

/**
 * 类别 → 文案键。写成**字面量表**而不是拼字符串 `t('feedback.cat.' + c)` ✗：
 * `t()` 的键是**穷举字面量类型** ✓，这样少一个键在**构建期**就报错 ✓
 * （拼字符串会把这个保护绕过去 ✗ —— 2026-10-11 补 `common.cancel` 时正是靠它在构建期拦下的 ✓）。
 */
const CATEGORY_LABEL: Record<
  FeedbackCategory,
  'feedback.cat.bug' | 'feedback.cat.suggest' | 'feedback.cat.experience' | 'feedback.cat.other'
> = {
  bug: 'feedback.cat.bug',
  suggest: 'feedback.cat.suggest',
  experience: 'feedback.cat.experience',
  other: 'feedback.cat.other',
}

/** 与服务端同一组数字（5~500 **字符** ✓）—— 这里只做 UX 提示，把关仍在服务端 ✓ */
const MIN = 5
const MAX = 500

/**
 * 功能反馈（侧栏用户卡片的第三个功能，2026-10-12）。
 *
 * 为什么要有它：用户遇到问题 / 有想法时，此前**没有任何站内入口** ✗（只能靠人找管理员 ✓）。
 * 现在提交后落库一张专表 `feedbacks` ✓，带上 `user_id`（能回溯、能回复 ✓），
 * 不混进埋点表（那是结构化事件 ✗）也不混进对话表（那些会被软删 ✓）。
 *
 * 前端只做**便宜的**检查（非空 + 字数 ✓，即时反馈 ✓）；长度真源、类别白名单、落库全在服务端 ✓。
 * ⚠️ 提交者身份**不经过请求体** ✗：`userId` 只由服务端从 Token 取 ✓，否则能替别人提反馈 ✓。
 */
export default function FeedbackModal({ open, onClose }: Props) {
  const [category, setCategory] = useState<FeedbackCategory>('bug')
  const [content, setContent] = useState('')
  const [contact, setContact] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [done, setDone] = useState(false)

  // 每次打开都是全新一次（上次的草稿不该在关掉再打开后还在 ✓）
  useEffect(() => {
    if (!open) return
    setCategory('bug')
    setContent('')
    setContact('')
    setBusy(false)
    setError('')
    setDone(false)
  }, [open])

  const reset = () => {
    setContent('')
    setContact('')
    setError('')
    setDone(false)
  }

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    const body = content.trim()
    if (body.length < MIN) {
      setError(t('feedback.tooShort'))
      return
    }
    setBusy(true)
    setError('')
    try {
      await feedbackApi.submit({
        category,
        content: body,
        contact: contact.trim() || undefined,
      })
      setDone(true)
      setContent('')
      setContact('')
    } catch (err) {
      setError(err instanceof Error ? err.message : t('errors.unknown'))
    } finally {
      setBusy(false)
    }
  }

  const inputClass =
    'mt-1 w-full rounded-xl border border-teyvat-line bg-white/[0.04] px-3 py-2 text-base text-teyvat-text outline-none transition placeholder:text-teyvat-text-4 focus:border-teyvat-line-strong focus:shadow-glow sm:text-sm'

  const left = content.trim().length

  return (
    <Modal
      open={open}
      title={t('feedback.title')}
      onClose={onClose}
      footer={
        done ? (
          <div className="flex items-center justify-end gap-2">
            <Button variant="ghost" size="sm" onClick={reset}>
              {t('feedback.again')}
            </Button>
            <Button variant="primary" size="sm" onClick={onClose}>
              {t('common.close')}
            </Button>
          </div>
        ) : (
          <div className="flex items-center justify-end gap-2">
            <Button variant="ghost" size="sm" onClick={onClose} disabled={busy}>
              {t('common.cancel')}
            </Button>
            <Button variant="primary" type="submit" form="feedback-form" disabled={busy}>
              {t('feedback.submit')}
            </Button>
          </div>
        )
      }
    >
      {done ? (
        <div className="space-y-2 py-2">
          <p className="text-xs text-ok">{t('feedback.ok')}</p>
          <p className="text-[11px] leading-relaxed text-teyvat-text-4">{t('feedback.okHint')}</p>
        </div>
      ) : (
        <form id="feedback-form" onSubmit={submit} className="space-y-4">
          <p className="text-[11px] leading-relaxed text-teyvat-text-4">{t('feedback.intro')}</p>

          <div>
            <label htmlFor="fb-category" className="text-xs text-teyvat-text-3">
              {t('feedback.category')}
            </label>
            <select
              id="fb-category"
              value={category}
              onChange={(e) => setCategory(e.target.value as FeedbackCategory)}
              className={inputClass}
            >
              {CATEGORIES.map((c) => (
                <option key={c} value={c}>
                  {t(CATEGORY_LABEL[c])}
                </option>
              ))}
            </select>
          </div>

          <div>
            <label htmlFor="fb-content" className="text-xs text-teyvat-text-3">
              {t('feedback.content')}
            </label>
            <textarea
              id="fb-content"
              rows={5}
              maxLength={MAX}
              value={content}
              onChange={(e) => setContent(e.target.value)}
              placeholder={t('feedback.placeholder')}
              className={`${inputClass} resize-y`}
            />
            <p className="mt-1 flex items-center justify-between text-[11px] leading-relaxed text-teyvat-text-4">
              <span>{t('feedback.rule')}</span>
              <span className={left > MAX ? 'text-err' : ''}>
                {left}/{MAX}
              </span>
            </p>
          </div>

          <div>
            <label htmlFor="fb-contact" className="text-xs text-teyvat-text-3">
              {t('feedback.contact')}
            </label>
            <input
              id="fb-contact"
              value={contact}
              onChange={(e) => setContact(e.target.value)}
              placeholder={t('feedback.contactPlaceholder')}
              className={inputClass}
            />
          </div>

          {error ? <p className="text-xs text-err">{error}</p> : null}
        </form>
      )}
    </Modal>
  )
}
