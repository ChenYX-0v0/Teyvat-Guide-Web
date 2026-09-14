import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { ChevronDown, Lock } from 'lucide-react'
import clsx from 'clsx'
import { useChatStore } from '@/store/useChatStore'
import { t } from '@/i18n'

/**
 * 匿名用户在对话页看到的内容：**预设问答**。
 *
 * 口径（见 docs/04-API接口文档.md §2.5）：匿名不再具备 AI 对话能力——
 * 这里不调用模型、不消耗 token，输入框只做展示并引导登录。
 */
export default function PresetQaPanel() {
  const { presets, presetsLoading, loadPresets } = useChatStore()
  // 手风琴：一次只看一条回答，避免长回答把页面撑得很难读
  const [openId, setOpenId] = useState<number | null>(null)

  useEffect(() => {
    if (presets.length === 0) void loadPresets()
  }, [presets.length, loadPresets])

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex-1 space-y-3 overflow-y-auto px-4 pb-4 sm:px-6 sm:pb-6">
        <p className="text-xs leading-relaxed text-white/40">{t('chat.presetHint')}</p>

        {presetsLoading && presets.length === 0 && (
          <p className="py-8 text-center text-sm text-white/35">{t('chat.presetTitle')}…</p>
        )}

        {!presetsLoading && presets.length === 0 && (
          <p className="py-8 text-center text-sm text-white/35">{t('chat.presetEmpty')}</p>
        )}

        {presets.map((item) => {
          const open = openId === item.id
          return (
            <div
              key={item.id}
              className="overflow-hidden rounded-xl border border-white/10 bg-white/5"
            >
              <button
                type="button"
                onClick={() => setOpenId(open ? null : item.id)}
                className="flex w-full items-center justify-between gap-3 px-4 py-3 text-left text-sm text-white/80 transition hover:bg-white/5"
              >
                <span className="min-w-0 truncate">{item.question}</span>
                <ChevronDown
                  className={clsx(
                    'h-4 w-4 shrink-0 text-white/40 transition',
                    open && 'rotate-180',
                  )}
                />
              </button>
              {open && (
                <div className="whitespace-pre-wrap border-t border-white/10 px-4 py-3 text-sm leading-relaxed text-white/70">
                  {item.answer}
                </div>
              )}
            </div>
          )
        })}
      </div>

      {/* 底部输入区：匿名不可用，只做登录引导（不让用户打完字才吃 401） */}
      <div className="border-t border-white/10 bg-black/25 p-4 pb-[max(1rem,env(safe-area-inset-bottom))] backdrop-blur">
        <div className="flex items-center gap-3">
          <div className="flex min-h-11 min-w-0 flex-1 items-center gap-2 rounded-xl border border-white/10 bg-white/5 px-4 text-base text-white/30 sm:text-sm">
            <Lock className="h-3.5 w-3.5 shrink-0" />
            <span className="truncate">{t('chat.loginRequired')}</span>
          </div>
          <Link
            to="/login"
            className="shrink-0 rounded-xl bg-teyvat-gold px-4 py-2.5 text-sm font-medium text-teyvat-ink transition hover:bg-teyvat-gold-deep"
          >
            {t('chat.presetLogin')}
          </Link>
        </div>
      </div>
    </div>
  )
}
