/**
 * 「角色养成度体检」面板的对话状态（真源：teyvat-docs/web/13-账号页悬浮AI分析（角色养成度）设计.md）
 *
 * 与对话页（`useChatStore`）的关系：**共用同一套接口与消息组件，但不共用会话**——
 * 体检用「每个 UID 一条独立会话」（D1 定案），所以面板只显示分析历史、不干扰主对话；
 * 这条会话是服务端真实会话，会正常出现在对话页的历史列表与后台对话记录里。
 *
 * 三条关键口径（§4）：
 *  1. 自动体检只在满足全部条件时触发：面板打开 + 已登录 + 有选中角色 + 该角色未体检过 + 当日未超软上限；
 *  2. **只有流正常结束（done 帧）才记「已体检」**——429 配额 / 网络失败 / 中途中断一律不记；
 *  3. 同一角色自动体检连续失败 2 次后，本次页面会话内不再自动重试（手动「重新分析」不受限）。
 */

import { useCallback, useEffect, useRef, useState } from 'react'
import { chatApi } from '@/api/chat'
import { t } from '@/i18n'
import { EVENTS, track } from '@/lib/track'
import { createAbortHandle, isAbortError, type AbortHandle } from '@/utils/abort'
import {
  analyzedCount,
  autoFailuresExceeded,
  canRunAutoToday,
  clearSessionId,
  isAnalyzed,
  markAnalyzed,
  markAutoFailure,
  markAutoToday,
  readSessionId,
  writeSessionId,
} from '@/lib/analysisGate'
import { getAuthToken } from '@/store/useAuthStore'
import type { ChatMessage, ChatSessionMessage } from '@/types'
import type { CharacterCard } from '@/types/genshin'

interface Options {
  /** 面板是否打开：关闭时不触发自动体检（已在跑的流不中断，结果照常落库） */
  enabled: boolean
  uid: string
  character?: CharacterCard
}

let seq = 0
const nextId = () => `analysis-${Date.now()}-${seq++}`

/** 服务端消息 → 视图消息（与对话页同一套口径） */
function toMessage(m: ChatSessionMessage): ChatMessage {
  const ts = Date.parse(m.createdAt)
  return {
    id: String(m.id),
    role: m.role,
    content: m.content,
    createdAt: Number.isNaN(ts) ? Date.now() : ts,
  }
}

/** 失败原因粗分：配额与其它分开，便于埋点看"是额度用完了还是网断了" */
function failureReason(message: string): 'quota' | 'failed' {
  return /额度|配额|超限|并发/.test(message) ? 'quota' : 'failed'
}

export function useAnalysisChat({ enabled, uid, character }: Options) {
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [loading, setLoading] = useState(false)
  const [restoring, setRestoring] = useState(false)
  const [error, setError] = useState('')
  const [dailyLimited, setDailyLimited] = useState(false)

  const sessionRef = useRef<string | undefined>(undefined)
  /** 停止句柄（与对话页共用同一套口径，见 utils/abort.ts） */
  const abortRef = useRef<AbortHandle | null>(null)
  abortRef.current ??= createAbortHandle()
  const abort = abortRef.current
  /** 同步防重：StrictMode 双跑 effect、连点、自动+手动撞车都从这里拦住 */
  const inflightRef = useRef(false)
  /** 本次"面板打开"内已自动尝试过的角色，避免失败后立刻无限重试 */
  const autoTriedRef = useRef('')

  // 恢复该 UID 的分析历史（换 UID / 首次挂载都会走一次）
  useEffect(() => {
    if (!uid) {
      sessionRef.current = undefined
      setMessages([])
      return
    }
    const saved = readSessionId(uid)
    sessionRef.current = saved
    if (!saved) {
      setMessages([])
      return
    }
    let cancelled = false
    setRestoring(true)
    chatApi
      .sessionMessages(saved, { limit: 30 })
      .then((data) => {
        if (!cancelled) setMessages(data.messages.map(toMessage))
      })
      .catch(() => {
        // 会话不存在 / 不属于自己 / 网络异常：只清本地指针，服务端数据不动（与对话页 openSession 同口径）
        if (cancelled) return
        clearSessionId(uid)
        sessionRef.current = undefined
        setMessages([])
      })
      .finally(() => {
        if (!cancelled) setRestoring(false)
      })
    return () => {
      cancelled = true
    }
  }, [uid])

  /** 面板关闭时清掉"本次打开已尝试"的账，下次打开可再试（§4.3） */
  useEffect(() => {
    if (!enabled) autoTriedRef.current = ''
  }, [enabled])

  const send = useCallback(
    async (text: string, opts: { auto?: boolean; characterId?: number } = {}) => {
      const content = text.trim()
      if (!content || inflightRef.current) return
      if (!getAuthToken()) {
        setError(t('analysis.needLogin'))
        return
      }

      const userMsg: ChatMessage = { id: nextId(), role: 'user', content, createdAt: Date.now() }
      const pendingMsg: ChatMessage = {
        id: nextId(),
        role: 'assistant',
        content: '',
        createdAt: Date.now(),
        pending: true,
      }
      inflightRef.current = true
      setMessages((m) => [...m, userMsg, pendingMsg])
      setLoading(true)
      setError('')

      const startedAt = Date.now()
      const signal = abort.begin()
      let ok = false

      try {
        await chatApi.stream(
          { sessionId: sessionRef.current, message: content },
          {
            signal,
            onSession: (sessionId) => {
              sessionRef.current = sessionId
              writeSessionId(uid, sessionId)
            },
            onDelta: (delta) => {
              setMessages((m) =>
                m.map((x) =>
                  x.id === pendingMsg.id ? { ...x, content: x.content + delta, pending: false } : x,
                ),
              )
            },
            onDone: (data) => {
              ok = true
              track(EVENTS.accountAnalysisEnd, {
                result: 'ok',
                elapsed_ms: data.elapsedMs,
                tokens: data.tokens,
              })
            },
          },
        )
        setMessages((m) => m.map((x) => (x.id === pendingMsg.id ? { ...x, pending: false } : x)))
      } catch (err) {
        // 用户主动停止 / 离开：保留已生成内容，不提示、不记账，只标「已停止生成」
        if (isAbortError(err)) {
          setMessages((m) =>
            m.map((x) => (x.id === pendingMsg.id ? { ...x, pending: false, stopped: true } : x)),
          )
          return
        }
        const message = err instanceof Error ? err.message : t('errors.paimonDistracted')
        setError(message)
        setMessages((m) =>
          m.map((x) => (x.id === pendingMsg.id ? { ...x, pending: false, error: message } : x)),
        )
        // 失败不记"已体检"（§4.3），但要记失败次数（连续 2 次停止自动重试）
        if (opts.auto && opts.characterId !== undefined) {
          markAutoFailure(uid, opts.characterId)
        }
        track(EVENTS.accountAnalysisEnd, {
          result: failureReason(message),
          elapsed_ms: Date.now() - startedAt,
        })
      } finally {
        abort.end()
        inflightRef.current = false
        setLoading(false)
      }

      // ✅ 唯一写"已体检"的位置：流正常结束
      if (ok && opts.characterId !== undefined) {
        markAnalyzed(uid, opts.characterId)
      }
    },
    [uid, abort],
  )

  // 自动体检：面板打开 + 有选中角色 + 该角色未体检过 + 当日未超上限
  useEffect(() => {
    if (!enabled || !uid || !character || restoring) return
    if (inflightRef.current || loading) return
    if (!getAuthToken()) return

    const id = character.id
    if (isAnalyzed(uid, id)) return
    if (autoFailuresExceeded(uid, id)) return
    if (!canRunAutoToday()) {
      setDailyLimited(true)
      return
    }
    const key = `${uid}:${id}`
    if (autoTriedRef.current === key) return

    autoTriedRef.current = key
    setDailyLimited(false)
    markAutoToday() // 记"尝试"而非"成功"：否则失败重试会绕过日上限
    track(EVENTS.accountAnalysisAuto, {
      character_id: id,
      is_first_char: analyzedCount(uid) === 0,
    })
    void send(t('analysis.question', { name: character.name }), { auto: true, characterId: id })
  }, [enabled, uid, character, restoring, loading, send])

  /** 手动重新分析：用户显式提问，不受"每角色一次"限制（也不改变已体检状态） */
  const rerun = useCallback(() => {
    if (!character) return
    track(EVENTS.accountAnalysisRerun, { character_id: character.id })
    void send(t('analysis.question', { name: character.name }))
  }, [character, send])

  /** 停止生成：掐断当前流（不可续）；已生成的内容保留并标「已停止生成」 */
  const stop = useCallback(() => abort.stop(), [abort])

  return { messages, loading, restoring, error, dailyLimited, send, rerun, stop }
}
