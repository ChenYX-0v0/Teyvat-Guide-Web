import type { ReactNode } from 'react'

/**
 * 行首分点识别（**纯文本协议，不是 Markdown**）。
 * 提示词 v11 第 15 条约定：分点用「1. 」「2. 」或「· 」开头、每条占一行。
 * 这里只认行首的这几种标记，不做任何 Markdown 解析（见 teyvat-docs/ai/07 TODO-17）。
 */
const POINT_LINE = /^(\d+[.、)]|[-·•*])\s+/

/**
 * 把派蒙的纯文本回答按行渲染：
 * - 行首是分点符号 ⇒ 悬挂缩进（长回答里 5~8 条也扫得动）
 * - 空行 ⇒ 保留一小段行距（原来是靠父容器 `whitespace-pre-wrap` 得到的）
 * - 其余行原样输出（行内空白仍由父容器 `whitespace-pre-wrap` 保留）
 *
 * ⚠️ 必须是**纯函数、每次按当前完整文本重算**：回答是流式增量拼接的，
 * 某一帧可能正好停在「1.」这种半行状态；任何"已渲染/已缩进"的缓存都会闪烁或漏缩进。
 *
 * @param content  当前完整回答文本
 * @param trailing 追加到最后一行末尾的节点（流式光标用，避免光标被挤到新的一行）
 */
export function renderAnswer(content: string, trailing?: ReactNode): ReactNode[] {
  const lines = content.split('\n')
  const lastIndex = lines.length - 1

  return lines.map((line, i) => {
    const isLast = i === lastIndex

    // 空行：给一小段固定行距（否则块级元素会把段落间的空行吃掉 ✗）
    if (line.trim() === '' && !isLast) {
      return <span key={i} className="block h-2" aria-hidden />
    }

    return (
      <span
        key={i}
        className="block"
        // 悬挂缩进：首行不缩进（跟正文对齐），折行的部分缩进 1.4em
        style={POINT_LINE.test(line) ? { paddingLeft: '1.4em', textIndent: '-1.4em' } : undefined}
      >
        {line}
        {isLast ? trailing : null}
      </span>
    )
  })
}
