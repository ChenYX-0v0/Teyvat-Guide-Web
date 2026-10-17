/**
 * 「停止生成」的共用能力（2026-10-17）
 *
 * 为什么单独抽出来：对话页（`useChatStore`）与角色体检面板（`useAnalysisChat`）
 * 要的是**同一套口径** —— 生成中一键掐断 SSE、保留已经吐出来的文字、可以立刻再提问。
 * 放两处各写一遍 AbortController 容易走偏（一处当失败、一处当取消），所以收在这里。
 *
 * 口径（**不可续**）：停止只是断开本次流，不做断点续写。
 * ⚠️ 服务端照旧会落库：`ChatService.AskStream` 在 `CompleteStream` 返回后
 * 无条件 `persistExchange`（已生成的部分 + Degraded 标记）⇒ 刷新会话能拿到同样的半截回答，
 * 前后端不会出现"界面有半截、重进就没了"的错位。
 */
export interface AbortHandle {
  /** 当前这轮流用的信号；没在生成时为 undefined */
  readonly signal: AbortSignal | undefined
  /** 开新一轮流（调用方自己保证不并发：上一次必须先 end 或 stop） */
  begin: () => AbortSignal
  /** 请求停止；没在生成时是空操作 */
  stop: () => void
  /** 这一轮结束（正常 / 失败 / 已停止都算），清掉引用避免误 stop */
  end: () => void
}

export function createAbortHandle(): AbortHandle {
  let ctrl: AbortController | undefined
  return {
    get signal() {
      return ctrl?.signal
    },
    begin() {
      ctrl = new AbortController()
      return ctrl.signal
    },
    stop() {
      ctrl?.abort()
    },
    end() {
      ctrl = undefined
    },
  }
}

/**
 * 区分「用户主动停止」与「真失败」：只有 AbortError 不算失败
 * （不弹错误文案、不记失败次数、不打失败埋点）。
 */
export function isAbortError(err: unknown): boolean {
  return err instanceof DOMException && err.name === 'AbortError'
}
