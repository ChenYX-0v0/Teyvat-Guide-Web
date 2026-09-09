import zhCN from './zh-CN'

/** 词典类型：以 zh-CN 为基准 */
export type Dict = typeof zhCN

/** 词典 key 的字面量联合类型，t() 的 key 有编译期检查 */
export type DictKey = keyof Dict

/** 支持的语言列表 */
export type Locale = 'zh-CN' | 'en'
