// 轻量 i18n：t() 取词 + 语言持久化
// 说明：当前 t() 在渲染时直接取词，切换语言后通过 setLocale() + 页面刷新生效；
// 后续如需无刷新切换，可将 locale 放入 Zustand 并让组件订阅（改动面仅此文件）。

import zhCN from './zh-CN'
import en from './en'
import type { Dict, DictKey, Locale } from './types'

export type { Dict, DictKey, Locale } from './types'

const dictionaries: Record<Locale, Partial<Dict>> = {
  'zh-CN': zhCN,
  en,
}

const STORAGE_KEY = 'app-locale'

function detectLocale(): Locale {
  const saved = localStorage.getItem(STORAGE_KEY)
  if (saved === 'zh-CN' || saved === 'en') return saved
  return navigator.language.toLowerCase().startsWith('zh') ? 'zh-CN' : 'en'
}

let currentLocale: Locale = detectLocale()

/** 当前语言 */
export function getLocale(): Locale {
  return currentLocale
}

/** 切换语言并持久化（刷新后生效） */
export function setLocale(locale: Locale): void {
  currentLocale = locale
  localStorage.setItem(STORAGE_KEY, locale)
}

/**
 * 取词
 * @param key 词典 key（编译期校验）
 * @param params 插值参数，替换文案中的 {name} 占位符
 */
export function t(key: DictKey, params?: Record<string, string | number>): string {
  let text = dictionaries[currentLocale][key] ?? zhCN[key] ?? key
  if (params) {
    for (const [name, value] of Object.entries(params)) {
      text = text.replaceAll(`{${name}}`, String(value))
    }
  }
  return text
}
