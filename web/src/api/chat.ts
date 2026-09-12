import { apiBaseURL, authHeader, http, unwrap } from './client'
import { t } from '@/i18n'
import type {
  ApiEnvelope,
  ChatRequest,
  ChatResponse,
  ChatStreamDone,
  ChatStreamHandlers,
  HealthData,
} from '@/types'

const decoder = new TextDecoder()

/** 解析单帧 data（服务端帧已被 JSON 序列化，解析失败视为协议异常） */
function decodeFrame<T>(raw: string): T {
  try {
    return JSON.parse(raw) as T
  } catch {
    // 帧格式不符合契约：抛出通用文案，避免把原始内容暴露给用户
    throw new Error(t('errors.requestFailed'))
  }
}

/** HTTP 层失败时尽量取出后端的统一信封文案 */
async function readErrorMessage(res: Response): Promise<string> {
  try {
    const body = (await res.json()) as ApiEnvelope<unknown>
    if (body?.message) return body.message
  } catch {
    // 非 JSON 响应（如代理错误页）：走通用文案
  }
  return t('errors.network')
}

export const chatApi = {
  send(payload: ChatRequest) {
    return unwrap<ChatResponse>(http.post('/chat', payload))
  },

  /**
   * 流式对话：以 fetch + ReadableStream 读取 SSE，逐段回调增量文本。
   * 帧格式：session → delta* → (done | error)，见 docs/04-API接口文档.md §2.1。
   * 失败（HTTP 非 2xx / error 帧 / 网络异常）时抛出，由调用方决定降级展示。
   */
  async stream(payload: ChatRequest, handlers: ChatStreamHandlers): Promise<void> {
    let res: Response
    try {
      res = await fetch(`${apiBaseURL}/chat`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', ...authHeader() },
        body: JSON.stringify({ ...payload, stream: true }),
        signal: handlers.signal,
      })
    } catch (err) {
      // 主动取消原样抛出，其余网络异常归一化
      if (err instanceof DOMException && err.name === 'AbortError') throw err
      throw new Error(t('errors.network'))
    }

    if (!res.ok) {
      throw new Error(await readErrorMessage(res))
    }
    if (!res.body) {
      throw new Error(t('errors.network'))
    }

    const reader = res.body.getReader()
    let buffer = ''
    let eventName = ''

    try {
      for (;;) {
        const { value, done } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })

        // 逐行消费完整帧；跨 chunk 被截断的行留在 buffer 里等下一段
        let idx = buffer.indexOf('\n')
        while (idx >= 0) {
          const line = buffer.slice(0, idx).replace(/\r$/, '')
          buffer = buffer.slice(idx + 1)
          idx = buffer.indexOf('\n')

          if (line.startsWith('event:')) {
            eventName = line.slice(6).trim()
          } else if (line.startsWith('data:')) {
            const raw = line.slice(5).trim()
            if (!raw) continue
            if (eventName === 'session') {
              handlers.onSession?.(decodeFrame<{ sessionId: string }>(raw).sessionId)
            } else if (eventName === 'delta') {
              handlers.onDelta(decodeFrame<{ text: string }>(raw).text)
            } else if (eventName === 'done') {
              handlers.onDone?.(decodeFrame<ChatStreamDone>(raw))
            } else if (eventName === 'error') {
              // 错误文案已由服务端脱敏，可直接展示
              const message = decodeFrame<{ message: string }>(raw).message
              throw new Error(message || t('errors.paimonDistracted'))
            }
          }
        }
      }
    } finally {
      // 正常结束时流已关闭，cancel 为幂等；异常时及时释放连接
      await reader.cancel().catch(() => undefined)
    }
  },

  health() {
    return unwrap<HealthData>(http.get('/health'))
  },
}
