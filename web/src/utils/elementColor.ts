/**
 * 元素名 → 元素色（token）。
 * 只用于**小面积标识**（徽记圆点），不做大面积铺色——这是视觉方案 §6.2 的约定。
 * 上游数据里元素名可能是「水」或「水元素」，这里统一去掉后缀再匹配。
 */
const ELEMENT_COLOR: Record<string, string> = {
  风: 'var(--color-el-anemo)',
  岩: 'var(--color-el-geo)',
  雷: 'var(--color-el-electro)',
  草: 'var(--color-el-dendro)',
  水: 'var(--color-el-hydro)',
  火: 'var(--color-el-pyro)',
  冰: 'var(--color-el-cryo)',
}

export function elementColor(element?: string): string | undefined {
  if (!element) return undefined
  return ELEMENT_COLOR[element.replace(/元素$/, '').trim()]
}
