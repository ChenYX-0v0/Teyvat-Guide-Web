import { useEffect, useRef, useState, type FormEvent } from 'react'
import { Check, ChevronDown } from 'lucide-react'
import clsx from 'clsx'
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
  // 「类型」下拉是否展开（**自制下拉**，不是原生 <select> —— 理由见下方注释 ✓）
  const [catOpen, setCatOpen] = useState(false)
  const catBoxRef = useRef<HTMLDivElement>(null)

  // 每次打开都是全新一次（上次的草稿不该在关掉再打开后还在 ✓）
  useEffect(() => {
    if (!open) return
    setCategory('bug')
    setContent('')
    setContact('')
    setBusy(false)
    setError('')
    setDone(false)
    setCatOpen(false)
  }, [open])

  // 下拉展开时：点外部关闭 ✓；Esc 先收下拉（**捕获阶段**截住，别让它同时把整个弹窗关掉 ✗）
  useEffect(() => {
    if (!catOpen) return
    const onDown = (e: MouseEvent) => {
      if (catBoxRef.current && !catBoxRef.current.contains(e.target as Node)) setCatOpen(false)
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
      // Modal 自己也监听 window 的 Esc（关整个弹窗）⇒ 这里在捕获阶段先拿住并截断，
      // 保证"下拉开着时 Esc 只收下拉" ✓（否则用户按 Esc 想收下拉，结果整个弹窗都没了 ✗）
      e.stopImmediatePropagation()
      setCatOpen(false)
    }
    window.addEventListener('mousedown', onDown)
    window.addEventListener('keydown', onKey, true)
    return () => {
      window.removeEventListener('mousedown', onDown)
      window.removeEventListener('keydown', onKey, true)
    }
  }, [catOpen])

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
    'mt-1 w-full rounded-xl border border-teyvat-line bg-teyvat-fill px-3 py-2 text-base text-teyvat-text outline-none transition placeholder:text-teyvat-text-4 focus:border-teyvat-line-strong focus:shadow-glow sm:text-sm'

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
            <span id="fb-cat-label" className="text-xs text-teyvat-text-3">
              {t('feedback.category')}
            </span>

            {/* ⚠️ 这里刻意**不用原生 `<select>`** ✗（2026-10-12 用户反馈"下拉是一片白"）：
                原生 select 的**收起态由系统控件绘制**，我们在它身上设的深色底/文字/边框基本被忽略 ✓；
                展开的候选项列表在 Windows 上也是系统白底 ✗（`<option>` 的样式各浏览器支持不一 ✗）。
                ⇒ 自制下拉，配色全部走项目主题令牌 ✓（浮层 L2 `teyvat-panel-soft` / 悬停 L3 `teyvat-raised`
                / 选中态金色，与侧栏浮层菜单同一套 ✓）。
                列表**走文档流**而不是绝对定位：Modal 的内容区是 `overflow-y-auto` ✓，
                绝对定位的浮层会被它裁掉 ✗；这里撑开几行反而更稳 ✓。
                ⚠️ 全站目前只有这一处下拉 ⇒ 先就地实现；出现第二处时再抽成 `common/Select` ✓。 */}
            <div ref={catBoxRef}>
              <button
                type="button"
                aria-haspopup="listbox"
                aria-expanded={catOpen}
                aria-labelledby="fb-cat-label"
                onClick={() => setCatOpen((v) => !v)}
                className={clsx(
                  'mt-1 flex w-full items-center justify-between rounded-xl border px-3 py-2 text-left text-base text-teyvat-text outline-none transition sm:text-sm',
                  catOpen
                    ? 'border-teyvat-line-strong bg-teyvat-fill shadow-glow'
                    : 'border-teyvat-line bg-teyvat-fill hover:border-teyvat-line-strong',
                )}
              >
                <span>{t(CATEGORY_LABEL[category])}</span>
                <ChevronDown
                  className={clsx('h-4 w-4 shrink-0 text-teyvat-text-3 transition', catOpen && 'rotate-180')}
                />
              </button>

              {catOpen ? (
                <ul
                  role="listbox"
                  aria-labelledby="fb-cat-label"
                  className="mt-1 overflow-hidden rounded-xl border border-teyvat-line-strong bg-teyvat-panel-soft shadow-panel"
                >
                  {CATEGORIES.map((c) => (
                    <li key={c}>
                      <button
                        type="button"
                        role="option"
                        aria-selected={c === category}
                        onClick={() => {
                          setCategory(c)
                          setCatOpen(false)
                        }}
                        className={clsx(
                          'flex w-full items-center justify-between px-3 py-2 text-left text-sm transition',
                          c === category
                            ? 'bg-teyvat-gold/10 text-teyvat-gold'
                            : 'text-teyvat-text-2 hover:bg-teyvat-raised hover:text-teyvat-text',
                        )}
                      >
                        {t(CATEGORY_LABEL[c])}
                        {c === category ? <Check className="h-3.5 w-3.5" /> : null}
                      </button>
                    </li>
                  ))}
                </ul>
              ) : null}
            </div>
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
